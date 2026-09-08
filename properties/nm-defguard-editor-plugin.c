#ifndef _GNU_SOURCE
#define _GNU_SOURCE
#endif

#include <NetworkManager.h>
#include <dlfcn.h>

#define DEFGUARD_SERVICE "org.freedesktop.NetworkManager.defguard"

typedef struct {
    GObject parent;
} DefguardEditorPlugin;

typedef struct {
    GObjectClass parent;
} DefguardEditorPluginClass;

typedef NMVpnEditor *(*EditorFactory)(NMVpnEditorPlugin *, NMConnection *,
                                      GError **);

static void editor_plugin_iface_init(NMVpnEditorPluginInterface *iface);

G_DEFINE_TYPE_EXTENDED(DefguardEditorPlugin, defguard_editor_plugin,
                       G_TYPE_OBJECT, 0,
                       G_IMPLEMENT_INTERFACE(NM_TYPE_VPN_EDITOR_PLUGIN,
                                             editor_plugin_iface_init))

enum {
    PROP_0,
    PROP_NAME,
    PROP_DESCRIPTION,
    PROP_SERVICE,
};

static void get_property(GObject *object, guint prop_id, GValue *value,
                         GParamSpec *spec) {
    switch (prop_id) {
    case PROP_NAME:
        g_value_set_string(value, "Defguard");
        break;
    case PROP_DESCRIPTION:
        g_value_set_string(value, "Defguard VPN");
        break;
    case PROP_SERVICE:
        g_value_set_string(value, DEFGUARD_SERVICE);
        break;
    default:
        G_OBJECT_WARN_INVALID_PROPERTY_ID(object, prop_id, spec);
    }
}

static void
defguard_editor_plugin_class_init(DefguardEditorPluginClass *klass) {
    GObjectClass *object_class = G_OBJECT_CLASS(klass);

    object_class->get_property = get_property;
    g_object_class_override_property(object_class, PROP_NAME,
                                     NM_VPN_EDITOR_PLUGIN_NAME);
    g_object_class_override_property(object_class, PROP_DESCRIPTION,
                                     NM_VPN_EDITOR_PLUGIN_DESCRIPTION);
    g_object_class_override_property(object_class, PROP_SERVICE,
                                     NM_VPN_EDITOR_PLUGIN_SERVICE);
}

static void defguard_editor_plugin_init(DefguardEditorPlugin *plugin) {
    (void)plugin;
}

static EditorFactory load_editor(GError **error) {
    static EditorFactory factory;
    static void *module;
    Dl_info info;
    const char *filename;
    char *directory;
    char *path;

    if (factory)
        return factory;

    filename = dlsym(RTLD_DEFAULT, "gtk_container_add")
                   ? "libnm-vpn-plugin-defguard-editor.so"
                   : "libnm-gtk4-vpn-plugin-defguard-editor.so";
    if (!dladdr(nm_vpn_editor_plugin_factory, &info) || !info.dli_fname) {
        g_set_error_literal(error, NM_VPN_PLUGIN_ERROR,
                            NM_VPN_PLUGIN_ERROR_FAILED,
                            "cannot locate the Defguard editor plugin");
        return NULL;
    }
    directory = g_path_get_dirname(info.dli_fname);
    path = g_build_filename(directory, filename, NULL);
    module = dlopen(path, RTLD_LAZY | RTLD_LOCAL);
    if (module)
        *(void **)(&factory) = dlsym(module, "nm_vpn_editor_factory_defguard");
    if (!factory)
        g_set_error(error, NM_VPN_PLUGIN_ERROR, NM_VPN_PLUGIN_ERROR_FAILED,
                    "cannot load %s: %s", path, dlerror());
    g_free(path);
    g_free(directory);
    return factory;
}

static NMVpnEditor *get_editor(NMVpnEditorPlugin *plugin,
                               NMConnection *connection, GError **error) {
    EditorFactory factory = load_editor(error);

    return factory ? factory(plugin, connection, error) : NULL;
}

static NMVpnEditorPluginCapability get_capabilities(NMVpnEditorPlugin *plugin) {
    (void)plugin;
    return NM_VPN_EDITOR_PLUGIN_CAPABILITY_NONE;
}

static void editor_plugin_iface_init(NMVpnEditorPluginInterface *iface) {
    iface->get_editor = get_editor;
    iface->get_capabilities = get_capabilities;
}

G_MODULE_EXPORT NMVpnEditorPlugin *
nm_vpn_editor_plugin_factory(GError **error) {
    (void)error;
    return NM_VPN_EDITOR_PLUGIN(
        g_object_new(defguard_editor_plugin_get_type(), NULL));
}
