//go:build darwin && cgo

#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#include <stdio.h>

static NSWindow *window;
static WKWebView *view;
static void emit(const char *event) { puts(event); fflush(stdout); }
static BOOL allowed(NSURL *url) {
    if ([url.absoluteString isEqualToString:@"about:blank"]) return YES;
    NSString *host=url.host.lowercaseString;
    return [url.scheme isEqualToString:@"http"] && !url.user && !url.password && url.port.intValue==8765 &&
      ([host isEqualToString:@"localhost"] || [host isEqualToString:@"127.0.0.1"] || [host isEqualToString:@"::1"] || [host isEqualToString:@"[::1]"]);
}
@interface FTCaptchaDelegate : NSObject<NSWindowDelegate,WKNavigationDelegate,WKUIDelegate>
@end
@implementation FTCaptchaDelegate
- (void)reload:(id)sender { [view reload]; }
- (void)external:(id)sender { emit("EXTERNAL"); [window close]; }
- (void)windowWillClose:(NSNotification *)notification {
    emit("CLOSED");
    window=nil; view=nil;
    [NSApp stop:nil];
    [NSApp postEvent:[NSEvent otherEventWithType:NSEventTypeApplicationDefined location:NSZeroPoint modifierFlags:0 timestamp:0 windowNumber:0 context:nil subtype:0 data1:0 data2:0] atStart:NO];
}
- (void)webView:(WKWebView *)webview decidePolicyForNavigationAction:(WKNavigationAction *)action decisionHandler:(void (^)(WKNavigationActionPolicy))handler {
    handler(allowed(action.request.URL) && action.targetFrame!=nil ? WKNavigationActionPolicyAllow : WKNavigationActionPolicyCancel);
}
- (void)webView:(WKWebView *)webview decidePolicyForNavigationResponse:(WKNavigationResponse *)response decisionHandler:(void (^)(WKNavigationResponsePolicy))handler {
    handler(response.canShowMIMEType ? WKNavigationResponsePolicyAllow : WKNavigationResponsePolicyCancel);
}
- (void)webView:(WKWebView *)webview didFailProvisionalNavigation:(WKNavigation *)navigation withError:(NSError *)error {
    if (error.code==NSURLErrorCancelled) return;
    emit("FAILED"); [window close];
}
- (void)webView:(WKWebView *)webview didFailNavigation:(WKNavigation *)navigation withError:(NSError *)error {
    if (error.code==NSURLErrorCancelled) return;
    emit("FAILED"); [window close];
}
- (WKWebView *)webView:(WKWebView *)webview createWebViewWithConfiguration:(WKWebViewConfiguration *)configuration forNavigationAction:(WKNavigationAction *)action windowFeatures:(WKWindowFeatures *)features { return nil; }
- (void)webView:(WKWebView *)webview runOpenPanelWithParameters:(WKOpenPanelParameters *)parameters initiatedByFrame:(WKFrameInfo *)frame completionHandler:(void (^)(NSArray<NSURL *> *))handler { handler(nil); }
- (void)webView:(WKWebView *)webview requestMediaCapturePermissionForOrigin:(WKSecurityOrigin *)origin initiatedByFrame:(WKFrameInfo *)frame type:(WKMediaCaptureType)type decisionHandler:(void (^)(WKPermissionDecision))handler { handler(WKPermissionDecisionDeny); }
@end

void ft_captcha_command(int command) {
    dispatch_async(dispatch_get_main_queue(), ^{
        if (command==1 && window) { [window deminiaturize:nil]; [NSApp activateIgnoringOtherApps:YES]; [window makeKeyAndOrderFront:nil]; }
        if (command==2 && window) [window close];
    });
}
int ft_captcha_run(const char *url, const char *title, const char *reload, const char *external) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        // A standalone task-switcher entry keeps verification recoverable
        // after the main FreeTurn app or another application takes focus.
        [NSApp setActivationPolicy:NSApplicationActivationPolicyRegular];
        FTCaptchaDelegate *delegate=[FTCaptchaDelegate new];
        WKWebViewConfiguration *config=[WKWebViewConfiguration new];
        config.websiteDataStore=[WKWebsiteDataStore nonPersistentDataStore];
        config.preferences.javaScriptCanOpenWindowsAutomatically=NO;
        window=[[NSWindow alloc] initWithContentRect:NSMakeRect(0,0,720,700) styleMask:(NSWindowStyleMaskTitled|NSWindowStyleMaskClosable|NSWindowStyleMaskResizable|NSWindowStyleMaskMiniaturizable) backing:NSBackingStoreBuffered defer:NO];
        window.title=[NSString stringWithUTF8String:title]; window.delegate=delegate;
        window.minSize=NSMakeSize(480,420);
        window.releasedWhenClosed=NO;
        window.hidesOnDeactivate=NO;
        window.appearance=[NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
        NSView *content=window.contentView;
        view=[[WKWebView alloc] initWithFrame:NSMakeRect(0,0,720,650) configuration:config];
        view.autoresizingMask=NSViewWidthSizable|NSViewHeightSizable;
        view.navigationDelegate=delegate; view.UIDelegate=delegate;
        [content addSubview:view];
        NSButton *reloadButton=[NSButton buttonWithTitle:[NSString stringWithUTF8String:reload] target:delegate action:@selector(reload:)];
        reloadButton.frame=NSMakeRect(10,659,160,30); reloadButton.autoresizingMask=NSViewMinYMargin;
        [content addSubview:reloadButton];
        NSButton *externalButton=[NSButton buttonWithTitle:[NSString stringWithUTF8String:external] target:delegate action:@selector(external:)];
        externalButton.frame=NSMakeRect(450,659,260,30); externalButton.autoresizingMask=NSViewMinXMargin|NSViewMinYMargin;
        [content addSubview:externalButton];
        [window center]; [window makeKeyAndOrderFront:nil]; [NSApp activateIgnoringOtherApps:YES];
        emit("READY");
        [view loadRequest:[NSURLRequest requestWithURL:[NSURL URLWithString:[NSString stringWithUTF8String:url]]]];
        [NSApp run];
        [delegate release]; [config release];
    }
    return 0;
}
