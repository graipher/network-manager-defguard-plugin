#include <NetworkManager.h>
#include <dlfcn.h>

#define DEFGUARD_SERVICE "org.freedesktop.NetworkManager.defguard"

int main(int argc, char **argv) {
    GError *error = NULL;
    NMVpnEditorPlugin *plugin;
    char *name = NULL;
    char *service = NULL;
    int i;

    g_test_init(&argc, &argv, NULL);
    g_assert_cmpint(argc, ==, 4);
    plugin = nm_vpn_editor_plugin_load_from_file(argv[1], DEFGUARD_SERVICE, -1,
                                                 NULL, NULL, &error);
    g_assert_no_error(error);
    g_assert_nonnull(plugin);
    g_object_get(plugin, NM_VPN_EDITOR_PLUGIN_NAME, &name,
                 NM_VPN_EDITOR_PLUGIN_SERVICE, &service, NULL);
    g_assert_cmpstr(name, ==, "Defguard");
    g_assert_cmpstr(service, ==, DEFGUARD_SERVICE);
    g_free(name);
    g_free(service);
    g_object_unref(plugin);
    for (i = 2; i < argc; i++) {
        void *module = dlopen(argv[i], RTLD_NOW | RTLD_LOCAL);

        g_assert_nonnull(module);
        g_assert_nonnull(dlsym(module, "nm_vpn_editor_factory_defguard"));
        dlclose(module);
    }
    return 0;
}
