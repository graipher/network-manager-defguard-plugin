PREFIX ?= /usr
LIBEXECDIR ?= $(PREFIX)/libexec
BUILD_DIR ?= build
NM_PLUGIN_DIR ?= $(shell pkg-config --variable=libdir libnm)/NetworkManager
CC ?= cc
CFLAGS ?= -O2 -Wall -Wextra
EDITOR_PLUGIN = $(BUILD_DIR)/libnm-vpn-plugin-defguard.so

.PHONY: build test install

build: $(EDITOR_PLUGIN) | $(BUILD_DIR)/.dir
	go build -buildvcs=false -o $(BUILD_DIR)/nm-defguard-service ./cmd/nm-defguard-service
	go build -buildvcs=false -o $(BUILD_DIR)/nm-defguard-import ./cmd/nm-defguard-import
	go build -buildvcs=false -o $(BUILD_DIR)/nm-defguard-auth-dialog ./cmd/nm-defguard-auth-dialog
	$(CC) $(CFLAGS) -fPIC -shared -Wl,-z,defs -o $(BUILD_DIR)/libnm-vpn-plugin-defguard-editor.so properties/nm-defguard-editor.c $$(pkg-config --cflags --libs libnm gtk+-3.0)
	$(CC) $(CFLAGS) -fPIC -shared -Wl,-z,defs -o $(BUILD_DIR)/libnm-gtk4-vpn-plugin-defguard-editor.so properties/nm-defguard-editor.c $$(pkg-config --cflags --libs libnm gtk4)

$(BUILD_DIR)/.dir:
	mkdir -p $(BUILD_DIR)
	touch $@

$(EDITOR_PLUGIN): properties/nm-defguard-editor-plugin.c | $(BUILD_DIR)/.dir
	$(CC) $(CFLAGS) -fPIC -shared -Wl,-z,defs -o $@ $< $$(pkg-config --cflags --libs libnm) -ldl

test: build
	go test -buildvcs=false ./...
	$(CC) $(CFLAGS) -o $(BUILD_DIR)/nm-defguard-editor-plugin-test properties/nm-defguard-editor-plugin-test.c $$(pkg-config --cflags --libs libnm) -ldl
	$(BUILD_DIR)/nm-defguard-editor-plugin-test $(abspath $(EDITOR_PLUGIN)) $(abspath $(BUILD_DIR)/libnm-vpn-plugin-defguard-editor.so) $(abspath $(BUILD_DIR)/libnm-gtk4-vpn-plugin-defguard-editor.so)

install:
	install -Dm755 $(BUILD_DIR)/nm-defguard-service $(DESTDIR)$(LIBEXECDIR)/nm-defguard-service
	install -Dm755 $(BUILD_DIR)/nm-defguard-import $(DESTDIR)$(PREFIX)/bin/nm-defguard-import
	install -Dm755 $(BUILD_DIR)/nm-defguard-auth-dialog $(DESTDIR)$(LIBEXECDIR)/nm-defguard-auth-dialog
	install -Dm755 $(BUILD_DIR)/libnm-vpn-plugin-defguard.so $(DESTDIR)$(NM_PLUGIN_DIR)/libnm-vpn-plugin-defguard.so
	install -Dm755 $(BUILD_DIR)/libnm-vpn-plugin-defguard-editor.so $(DESTDIR)$(NM_PLUGIN_DIR)/libnm-vpn-plugin-defguard-editor.so
	install -Dm755 $(BUILD_DIR)/libnm-gtk4-vpn-plugin-defguard-editor.so $(DESTDIR)$(NM_PLUGIN_DIR)/libnm-gtk4-vpn-plugin-defguard-editor.so
	install -Dm644 packaging/NetworkManager/VPN/nm-defguard-service.name $(DESTDIR)$(PREFIX)/lib/NetworkManager/VPN/nm-defguard-service.name
	install -Dm644 packaging/dbus-1/system.d/nm-defguard-service.conf $(DESTDIR)$(PREFIX)/share/dbus-1/system.d/nm-defguard-service.conf
