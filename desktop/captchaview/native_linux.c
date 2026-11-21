//go:build linux && cgo

#include <gtk/gtk.h>
#include <webkit2/webkit2.h>
#include <stdio.h>
#include <string.h>

static GtkWidget *window;
static WebKitWebView *view;
static void emit(const char *event) { puts(event); fflush(stdout); }
static gboolean allowed(const char *uri) {
    if (!strcmp(uri,"about:blank")) return TRUE;
    GUri *u = g_uri_parse(uri, G_URI_FLAGS_NONE, NULL);
    if (!u) return FALSE;
    const char *host = g_uri_get_host(u), *scheme = g_uri_get_scheme(u);
    gboolean ok = scheme && !strcmp(scheme,"http") && !g_uri_get_userinfo(u) && g_uri_get_port(u)==8765 && host &&
        (!g_ascii_strcasecmp(host,"localhost") || !strcmp(host,"127.0.0.1") || !strcmp(host,"::1"));
    g_uri_unref(u);
    return ok;
}
static void close_window(void) { if (window) gtk_widget_destroy(window); }
static void destroyed(GtkWidget *widget, gpointer data) { window=NULL; view=NULL; emit("CLOSED"); gtk_main_quit(); }
static void reload_clicked(GtkButton *button, gpointer data) { if (view) webkit_web_view_reload(view); }
static void external_clicked(GtkButton *button, gpointer data) { emit("EXTERNAL"); close_window(); }
static gboolean load_failed(WebKitWebView *webview, WebKitLoadEvent event, const char *uri, GError *error, gpointer data) {
    // Cancellation is also reported when the user closes/reloads a page.
    if (g_error_matches(error, WEBKIT_NETWORK_ERROR, WEBKIT_NETWORK_ERROR_CANCELLED)) return TRUE;
    if (g_error_matches(error, WEBKIT_POLICY_ERROR, WEBKIT_POLICY_ERROR_FRAME_LOAD_INTERRUPTED_BY_POLICY_CHANGE)) return TRUE;
    emit("FAILED"); close_window(); return TRUE;
}
static gboolean decide(WebKitWebView *webview, WebKitPolicyDecision *decision, WebKitPolicyDecisionType type, gpointer data) {
    if (type==WEBKIT_POLICY_DECISION_TYPE_NAVIGATION_ACTION || type==WEBKIT_POLICY_DECISION_TYPE_NEW_WINDOW_ACTION) {
        WebKitNavigationAction *action = webkit_navigation_policy_decision_get_navigation_action(WEBKIT_NAVIGATION_POLICY_DECISION(decision));
        const char *uri = webkit_uri_request_get_uri(webkit_navigation_action_get_request(action));
        if (!allowed(uri) || type==WEBKIT_POLICY_DECISION_TYPE_NEW_WINDOW_ACTION) {
            webkit_policy_decision_ignore(decision); return TRUE;
        }
    } else if (type==WEBKIT_POLICY_DECISION_TYPE_RESPONSE && !webkit_response_policy_decision_is_mime_type_supported(WEBKIT_RESPONSE_POLICY_DECISION(decision))) {
        webkit_policy_decision_ignore(decision); return TRUE;
    }
    return FALSE;
}
static gboolean deny_permission(WebKitWebView *webview, WebKitPermissionRequest *request, gpointer data) { webkit_permission_request_deny(request); return TRUE; }
static void deny_download(WebKitWebContext *context, WebKitDownload *download, gpointer data) { webkit_download_cancel(download); }
static gboolean deny_upload(WebKitWebView *webview, WebKitFileChooserRequest *request, gpointer data) { webkit_file_chooser_request_cancel(request); return TRUE; }
static gboolean command_on_main(gpointer data) {
    int command=GPOINTER_TO_INT(data);
    if (window && command==1) gtk_window_present(GTK_WINDOW(window));
    if (command==2) close_window();
    return G_SOURCE_REMOVE;
}
void ft_captcha_command(int command) { g_idle_add(command_on_main,GINT_TO_POINTER(command)); }
int ft_captcha_run(const char *url, const char *title, const char *reload, const char *external) {
    g_set_prgname("freeturn");
    if (!gtk_init_check(NULL,NULL)) return 1;
    g_object_set(gtk_settings_get_default(),"gtk-application-prefer-dark-theme",TRUE,NULL);
    WebKitWebContext *context=webkit_web_context_new_ephemeral();
    g_signal_connect(context,"download-started",G_CALLBACK(deny_download),NULL);
    window=gtk_window_new(GTK_WINDOW_TOPLEVEL);
    gtk_window_set_title(GTK_WINDOW(window),title);
    gtk_window_set_default_size(GTK_WINDOW(window),720,700);
    gtk_widget_set_size_request(window,480,420);
    gtk_window_set_position(GTK_WINDOW(window),GTK_WIN_POS_CENTER);
    g_signal_connect(window,"destroy",G_CALLBACK(destroyed),NULL);
    GtkWidget *box=gtk_box_new(GTK_ORIENTATION_VERTICAL,0), *toolbar=gtk_box_new(GTK_ORIENTATION_HORIZONTAL,8);
    gtk_container_set_border_width(GTK_CONTAINER(toolbar),8);
    GtkWidget *reload_button=gtk_button_new_with_label(reload), *external_button=gtk_button_new_with_label(external);
    g_signal_connect(reload_button,"clicked",G_CALLBACK(reload_clicked),NULL);
    g_signal_connect(external_button,"clicked",G_CALLBACK(external_clicked),NULL);
    gtk_box_pack_start(GTK_BOX(toolbar),reload_button,FALSE,FALSE,0);
    gtk_box_pack_end(GTK_BOX(toolbar),external_button,FALSE,FALSE,0);
    gtk_box_pack_start(GTK_BOX(box),toolbar,FALSE,FALSE,0);
    view=WEBKIT_WEB_VIEW(webkit_web_view_new_with_context(context));
    g_object_unref(context);
    WebKitSettings *settings=webkit_web_view_get_settings(view);
    webkit_settings_set_enable_developer_extras(settings,FALSE);
    webkit_settings_set_javascript_can_open_windows_automatically(settings,FALSE);
    GdkRGBA background={14.0/255,15.0/255,20.0/255,1};
    webkit_web_view_set_background_color(view,&background);
    g_signal_connect(view,"decide-policy",G_CALLBACK(decide),NULL);
    g_signal_connect(view,"permission-request",G_CALLBACK(deny_permission),NULL);
    g_signal_connect(view,"run-file-chooser",G_CALLBACK(deny_upload),NULL);
    g_signal_connect(view,"load-failed",G_CALLBACK(load_failed),NULL);
    gtk_box_pack_start(GTK_BOX(box),GTK_WIDGET(view),TRUE,TRUE,0);
    gtk_container_add(GTK_CONTAINER(window),box);
    gtk_widget_show_all(window);
    emit("READY");
    webkit_web_view_load_uri(view,url);
    gtk_main();
    return 0;
}
