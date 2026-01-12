package downloader

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mgomes/dl"
	"github.com/mgomes/putsweep/internal/config"
	"github.com/mgomes/putsweep/internal/putio"
	"github.com/mgomes/putsweep/internal/scheduler"
)

type Status int

const (
	StatusPending Status = iota
	StatusDownloading
	StatusCompleted
	StatusFailed
)

func (s Status) String() string {
	switch s {
	case StatusPending:
		return "Pending"
	case StatusDownloading:
		return "Downloading"
	case StatusCompleted:
		return "Completed"
	case StatusFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

type QueueItem struct {
	ID         string
	FileID     int64
	Name       string
	Size       uint64
	Status     Status
	Downloaded uint64
	AddedAt    time.Time
	Error      string
}

type ProgressUpdate struct {
	ItemID     string
	Downloaded uint64
	Total      uint64
	Speed      float64
}

type EventType int

const (
	EventDownloadStarted EventType = iota
	EventDownloadCompleted
	EventDownloadFailed
	EventQueueUpdated
)

type Event struct {
	Type    EventType
	ItemID  string
	Message string
}

type Manager struct {
	config      *config.Config
	putioClient *putio.Client
	scheduler   *scheduler.Scheduler

	queue     []*QueueItem
	completed []*QueueItem
	current   *QueueItem

	ctx    context.Context
	cancel context.CancelFunc

	mu sync.RWMutex

	progressCh chan ProgressUpdate
	eventCh    chan Event

	lastSpeed     float64
	lastSpeedTime time.Time
	lastBytes     uint64
}

func NewManager(cfg *config.Config, putioClient *putio.Client) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config:      cfg,
		putioClient: putioClient,
		scheduler:   scheduler.New(cfg.ScheduleMode),
		queue:       make([]*QueueItem, 0),
		completed:   make([]*QueueItem, 0),
		ctx:         ctx,
		cancel:      cancel,
		progressCh:  make(chan ProgressUpdate, 100),
		eventCh:     make(chan Event, 100),
	}
}

func (m *Manager) Start() {
	go m.run()
}

func (m *Manager) Stop() {
	m.cancel()
}

func (m *Manager) AddToQueue(urlStr string) error {
	fileID, err := putio.ParsePutioURL(urlStr)
	if err != nil {
		return err
	}

	fileInfo, err := m.putioClient.GetFileInfo(m.ctx, fileID)
	if err != nil {
		return err
	}

	item := &QueueItem{
		ID:      uuid.New().String(),
		FileID:  fileID,
		Name:    fileInfo.Name,
		Size:    fileInfo.Size,
		Status:  StatusPending,
		AddedAt: time.Now(),
	}

	m.mu.Lock()
	m.queue = append(m.queue, item)
	m.mu.Unlock()

	m.eventCh <- Event{Type: EventQueueUpdated}

	return nil
}

func (m *Manager) Queue() []*QueueItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*QueueItem, len(m.queue))
	copy(result, m.queue)
	return result
}

func (m *Manager) Completed() []*QueueItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*QueueItem, len(m.completed))
	copy(result, m.completed)
	return result
}

func (m *Manager) Current() *QueueItem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

func (m *Manager) ProgressChannel() <-chan ProgressUpdate {
	return m.progressCh
}

func (m *Manager) EventChannel() <-chan Event {
	return m.eventCh
}

func (m *Manager) Scheduler() *scheduler.Scheduler {
	return m.scheduler
}

func (m *Manager) SetScheduleMode(mode config.ScheduleMode) {
	m.scheduler.SetMode(mode)
	m.config.ScheduleMode = mode
}

func (m *Manager) run() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.process()
		}
	}
}

func (m *Manager) process() {
	m.mu.Lock()

	if m.current != nil && m.current.Status == StatusDownloading {
		m.mu.Unlock()
		return
	}

	if !m.scheduler.CanStartNew() {
		m.mu.Unlock()
		return
	}

	var nextItem *QueueItem
	for _, item := range m.queue {
		if item.Status == StatusPending {
			nextItem = item
			break
		}
	}

	if nextItem == nil {
		m.mu.Unlock()
		return
	}

	nextItem.Status = StatusDownloading
	m.current = nextItem
	m.mu.Unlock()

	go m.download(nextItem)
}

func (m *Manager) download(item *QueueItem) {
	m.eventCh <- Event{Type: EventDownloadStarted, ItemID: item.ID}

	downloadURL, err := m.putioClient.GetDownloadURL(m.ctx, item.FileID)
	if err != nil {
		m.failItem(item, err)
		return
	}

	reporter := &tuiProgressReporter{
		itemID:     item.ID,
		progressCh: m.progressCh,
		manager:    m,
	}

	downloader := &dl.Downloader{
		URI:        downloadURL,
		Filename:   item.Name,
		WorkingDir: m.config.DownloadDir,
		Boost:      m.config.Boost,
		Retries:    3,
		Resume:     true,
		Progress:   reporter,
		Context:    m.ctx,
	}

	if err := downloader.FetchMetadata(); err != nil {
		m.failItem(item, err)
		return
	}

	if err := downloader.Fetch(); err != nil {
		if errors.Is(err, context.Canceled) {
			m.mu.Lock()
			item.Status = StatusPending
			m.current = nil
			m.mu.Unlock()
		} else {
			m.failItem(item, err)
		}
		return
	}

	m.mu.Lock()
	item.Status = StatusCompleted
	item.Downloaded = item.Size
	m.current = nil

	newQueue := make([]*QueueItem, 0, len(m.queue)-1)
	for _, q := range m.queue {
		if q.ID != item.ID {
			newQueue = append(newQueue, q)
		}
	}
	m.queue = newQueue
	m.completed = append(m.completed, item)
	m.mu.Unlock()

	m.eventCh <- Event{Type: EventDownloadCompleted, ItemID: item.ID}
	m.eventCh <- Event{Type: EventQueueUpdated}
}

func (m *Manager) failItem(item *QueueItem, err error) {
	m.mu.Lock()
	item.Status = StatusFailed
	item.Error = err.Error()
	m.current = nil
	m.mu.Unlock()

	m.eventCh <- Event{Type: EventDownloadFailed, ItemID: item.ID, Message: err.Error()}
}

func (m *Manager) updateSpeed(downloaded uint64) float64 {
	now := time.Now()

	if m.lastSpeedTime.IsZero() {
		m.lastSpeedTime = now
		m.lastBytes = downloaded
		return 0
	}

	elapsed := now.Sub(m.lastSpeedTime).Seconds()
	if elapsed < 0.5 {
		return m.lastSpeed
	}

	bytesDiff := downloaded - m.lastBytes
	speed := float64(bytesDiff) / elapsed

	m.lastSpeed = speed
	m.lastSpeedTime = now
	m.lastBytes = downloaded

	return speed
}

type tuiProgressReporter struct {
	itemID     string
	progressCh chan<- ProgressUpdate
	total      uint64
	downloaded uint64
	manager    *Manager
	mu         sync.Mutex
}

func (r *tuiProgressReporter) SetTotal(total uint64) {
	r.mu.Lock()
	r.total = total
	r.mu.Unlock()
}

func (r *tuiProgressReporter) SetDownloaded(downloaded uint64) {
	r.mu.Lock()
	r.downloaded = downloaded
	speed := r.manager.updateSpeed(downloaded)
	r.mu.Unlock()

	r.progressCh <- ProgressUpdate{
		ItemID:     r.itemID,
		Downloaded: downloaded,
		Total:      r.total,
		Speed:      speed,
	}
}

func (r *tuiProgressReporter) AddDownloaded(delta uint64) {
	r.mu.Lock()
	r.downloaded += delta
	downloaded := r.downloaded
	speed := r.manager.updateSpeed(downloaded)
	r.mu.Unlock()

	r.progressCh <- ProgressUpdate{
		ItemID:     r.itemID,
		Downloaded: downloaded,
		Total:      r.total,
		Speed:      speed,
	}
}

func (r *tuiProgressReporter) Done() {
	r.mu.Lock()
	r.downloaded = r.total
	r.mu.Unlock()
}
