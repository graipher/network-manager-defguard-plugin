#include <NetworkManager.h>
#include <gtk/gtk.h>

typedef struct {
    GObject parent;
    GtkWidget *widget;
} DefguardEditor;

typedef struct {
    GObjectClass parent;
} DefguardEditorClass;

static void editor_iface_init(NMVpnEditorInterface *iface);

G_DEFINE_TYPE_EXTENDED(DefguardEditor, defguard_editor, G_TYPE_OBJECT, 0,
                       G_IMPLEMENT_INTERFACE(NM_TYPE_VPN_EDITOR,
                                             editor_iface_init))

static void box_append(GtkWidget *box, GtkWidget *child) {
#if GTK_CHECK_VERSION(4, 0, 0)
    gtk_box_append(GTK_BOX(box), child);
#else
    gtk_box_pack_start(GTK_BOX(box), child, FALSE, FALSE, 0);
#endif
}

static void frame_set_child(GtkWidget *frame, GtkWidget *child) {
#if GTK_CHECK_VERSION(4, 0, 0)
    gtk_frame_set_child(GTK_FRAME(frame), child);
#else
    gtk_container_add(GTK_CONTAINER(frame), child);
#endif
}

static void label_set_wrap(GtkWidget *label) {
#if GTK_CHECK_VERSION(4, 0, 0)
    gtk_label_set_wrap(GTK_LABEL(label), TRUE);
#else
    gtk_label_set_line_wrap(GTK_LABEL(label), TRUE);
#endif
}

static GtkWidget *value_label(const char *value) {
    GtkWidget *label = gtk_label_new(value && *value ? value : "—");

    gtk_label_set_xalign(GTK_LABEL(label), 0);
    gtk_label_set_selectable(GTK_LABEL(label), TRUE);
    gtk_widget_set_hexpand(label, TRUE);
    return label;
}

static void add_row(GtkWidget *grid, int row, const char *name,
                    const char *value) {
    GtkWidget *label = gtk_label_new(name);

    gtk_label_set_xalign(GTK_LABEL(label), 0);
    gtk_widget_set_halign(label, GTK_ALIGN_START);
    gtk_grid_attach(GTK_GRID(grid), label, 0, row, 1, 1);
    gtk_grid_attach(GTK_GRID(grid), value_label(value), 1, row, 1, 1);
}

static const char *data_item(NMSettingVpn *setting, const char *key) {
    const char *value =
        setting ? nm_setting_vpn_get_data_item(setting, key) : NULL;

    return value ? value : "";
}

static const char *traffic_label(const char *mode) {
    if (g_strcmp0(mode, "all") == 0 || !mode || !*mode)
        return "All traffic";
    if (g_strcmp0(mode, "predefined") == 0)
        return "Predefined traffic only";
    return mode;
}

static void defguard_editor_dispose(GObject *object) {
    DefguardEditor *editor = (DefguardEditor *)object;

    g_clear_object(&editor->widget);
    G_OBJECT_CLASS(defguard_editor_parent_class)->dispose(object);
}

static void defguard_editor_class_init(DefguardEditorClass *klass) {
    G_OBJECT_CLASS(klass)->dispose = defguard_editor_dispose;
}

static void defguard_editor_init(DefguardEditor *editor) { (void)editor; }

static GObject *get_widget(NMVpnEditor *editor) {
    return G_OBJECT(((DefguardEditor *)editor)->widget);
}

static gboolean update_connection(NMVpnEditor *editor, NMConnection *connection,
                                  GError **error) {
    (void)editor;
    (void)connection;
    (void)error;
    return TRUE;
}

static void editor_iface_init(NMVpnEditorInterface *iface) {
    iface->get_widget = get_widget;
    iface->update_connection = update_connection;
}

G_MODULE_EXPORT NMVpnEditor *
nm_vpn_editor_factory_defguard(NMVpnEditorPlugin *plugin,
                               NMConnection *connection, GError **error) {
    DefguardEditor *editor;
    NMSettingVpn *vpn;
    GtkWidget *frame;
    GtkWidget *box;
    GtkWidget *grid;
    GtkWidget *note;
    const char *mode;

    (void)plugin;
    (void)error;
    editor = g_object_new(defguard_editor_get_type(), NULL);
    vpn = nm_connection_get_setting_vpn(connection);

    box = gtk_box_new(GTK_ORIENTATION_VERTICAL, 12);
    gtk_widget_set_margin_top(box, 12);
    gtk_widget_set_margin_bottom(box, 12);
    gtk_widget_set_margin_start(box, 12);
    gtk_widget_set_margin_end(box, 12);
    editor->widget = g_object_ref_sink(box);

    frame = gtk_frame_new("Defguard connection");
    grid = gtk_grid_new();
    gtk_grid_set_row_spacing(GTK_GRID(grid), 8);
    gtk_grid_set_column_spacing(GTK_GRID(grid), 18);
    gtk_widget_set_margin_top(grid, 12);
    gtk_widget_set_margin_bottom(grid, 12);
    gtk_widget_set_margin_start(grid, 12);
    gtk_widget_set_margin_end(grid, 12);
    mode = data_item(vpn, "traffic-mode");
    add_row(grid, 0, "Location ID", data_item(vpn, "location-id"));
    add_row(grid, 1, "Instance", data_item(vpn, "instance"));
    add_row(grid, 2, "Endpoint", data_item(vpn, "endpoint"));
    add_row(grid, 3, "Traffic mode", traffic_label(mode));
    frame_set_child(frame, grid);
    box_append(box, frame);

    note = gtk_label_new(
        "Authentication, WireGuard settings, routes, and DNS are managed by "
        "Defguard. Run nm-defguard-import to change this profile.");
    gtk_label_set_xalign(GTK_LABEL(note), 0);
    label_set_wrap(note);
    box_append(box, note);

    return NM_VPN_EDITOR(editor);
}
