PREFIX ?= /usr
LIBEXECDIR ?= $(PREFIX)/libexec
BUILD_DIR ?= build

.PHONY: build test install

build:
	go build -buildvcs=false -o $(BUILD_DIR)/nm-defguard-service ./cmd/nm-defguard-service
	go build -buildvcs=false -o $(BUILD_DIR)/nm-defguard-import ./cmd/nm-defguard-import

test:
	go test -buildvcs=false ./...

install:
	install -Dm755 $(BUILD_DIR)/nm-defguard-service $(DESTDIR)$(LIBEXECDIR)/nm-defguard-service
	install -Dm755 $(BUILD_DIR)/nm-defguard-import $(DESTDIR)$(PREFIX)/bin/nm-defguard-import
	install -Dm644 packaging/NetworkManager/VPN/nm-defguard-service.name $(DESTDIR)$(PREFIX)/lib/NetworkManager/VPN/nm-defguard-service.name
	install -Dm644 packaging/dbus-1/system.d/nm-defguard-service.conf $(DESTDIR)$(PREFIX)/share/dbus-1/system.d/nm-defguard-service.conf
