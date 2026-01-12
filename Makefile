BINARY_NAME=putsweep
BUILD_DIR=.
CMD_DIR=./cmd/putsweep

.PHONY: build clean install uninstall run tidy

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

clean:
	rm -f $(BUILD_DIR)/$(BINARY_NAME)

install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/

uninstall:
	rm -f /usr/local/bin/$(BINARY_NAME)

run: build
	$(BUILD_DIR)/$(BINARY_NAME)

tidy:
	go mod tidy
