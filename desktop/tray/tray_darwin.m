//go:build darwin
#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>
#include <stdatomic.h>
static void applyStatusIcon(void);
extern void freeturn_tray_action(int action);
@interface FreeTurnTrayTarget : NSObject
- (void)show:(id)sender;
- (void)quit:(id)sender;
@end
@implementation FreeTurnTrayTarget
- (void)show:(id)sender { freeturn_tray_action(1); }
- (void)quit:(id)sender { freeturn_tray_action(2); }
- (void)observeValueForKeyPath:(NSString *)keyPath ofObject:(id)object change:(NSDictionary *)change context:(void *)context {
  if([keyPath isEqualToString:@"effectiveAppearance"]) applyStatusIcon();
  else [super observeValueForKeyPath:keyPath ofObject:object change:change context:context];
}
@end
static NSStatusItem *statusItem;
static FreeTurnTrayTarget *target;
static id launchObserver;
static id motionObserver;
static NSData *darkStatusData,*lightStatusData;
static BOOL statusTemplate=YES;
static BOOL watchingAppearance=NO;
static atomic_bool reducedMotion=ATOMIC_VAR_INIT(1);

static void applyStatusIcon(void) {
 if(!statusItem || !darkStatusData) return;
 NSImage *dark=[[NSImage alloc] initWithData:darkStatusData],*light=[[NSImage alloc] initWithData:lightStatusData];
 if(!dark || !light) return;
 NSImage *image=dark;
 if(!statusTemplate) {
  image=[NSImage imageWithSize:NSMakeSize(20,20) flipped:NO drawingHandler:^BOOL(NSRect rect){
   NSString *appearance=[statusItem.button.effectiveAppearance bestMatchFromAppearancesWithNames:@[NSAppearanceNameAqua,NSAppearanceNameDarkAqua]];
   NSImage *source=[appearance isEqualToString:NSAppearanceNameDarkAqua]?light:dark;
   [source drawInRect:rect fromRect:NSZeroRect operation:NSCompositingOperationSourceOver fraction:1];
   return YES;
  }];
  // Choose contrast at drawing time as well as on effectiveAppearance changes.
  image.cacheMode=NSImageCacheNever;
 }
 image.size=NSMakeSize(20,20);
 [image setTemplate:statusTemplate];
 statusItem.button.image=image;
 statusItem.button.imagePosition=NSImageOnly;
 statusItem.button.title=@"";
}
// Wails v2 has no reopen callback. Add the optional AppKit delegate method
// without replacing the delegate or any implementation supplied by Wails.
static BOOL handleDockReopen(id delegate, SEL selector, NSApplication *app, BOOL hasVisibleWindows) {
 freeturn_tray_action(1);
 return NO; // ShowWindow handles showing, activation and deminiaturization.
}
static void installDockReopenHandler(id delegate) {
 if(!delegate) return;
 Class cls=object_getClass(delegate);
 SEL selector=@selector(applicationShouldHandleReopen:hasVisibleWindows:);
 if(class_getInstanceMethod(cls,selector)) return;
 struct objc_method_description method=protocol_getMethodDescription(@protocol(NSApplicationDelegate),selector,NO,YES);
 if(method.types) class_addMethod(cls,selector,(IMP)handleDockReopen,method.types);
}
static void bindDockReopen(void) { installDockReopenHandler(NSApp.delegate); }

// Wails routes the native Quit role through OnBeforeClose. When close-to-tray
// is enabled that would hide the app instead of quitting. Route Cmd+Q through
// the same explicit Quit callback as the status-menu action.
static void bindQuitShortcut(void) {
 for (NSMenuItem *root in NSApp.mainMenu.itemArray) {
  for (NSMenuItem *item in root.submenu.itemArray) {
   if ([item.keyEquivalent.lowercaseString isEqualToString:@"q"] && (item.keyEquivalentModifierMask & NSEventModifierFlagCommand)) {
    item.target=target; item.action=@selector(quit:);
   }
  }
 }
}
void freeturn_tray_start(const char *title, const char *open, const char *quit, const unsigned char *icon, int length) {
  NSString *name=[NSString stringWithUTF8String:title],*openTitle=[NSString stringWithUTF8String:open],*quitTitle=[NSString stringWithUTF8String:quit];
  // Copy Go-owned bytes before scheduling asynchronous AppKit work.
  NSData *iconData=[NSData dataWithBytes:icon length:(NSUInteger)length];
 dispatch_async(dispatch_get_main_queue(), ^{
  target=[FreeTurnTrayTarget new];
  launchObserver=[[NSNotificationCenter defaultCenter] addObserverForName:NSApplicationDidFinishLaunchingNotification object:nil queue:[NSOperationQueue mainQueue] usingBlock:^(NSNotification *note) { bindQuitShortcut(); bindDockReopen(); }];
  bindQuitShortcut();
  bindDockReopen();
   statusItem=[[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];
   darkStatusData=iconData;lightStatusData=iconData;statusTemplate=YES;
   [statusItem.button addObserver:target forKeyPath:@"effectiveAppearance" options:0 context:NULL];watchingAppearance=YES;
   atomic_store(&reducedMotion,[NSWorkspace sharedWorkspace].accessibilityDisplayShouldReduceMotion);
   motionObserver=[[[NSWorkspace sharedWorkspace] notificationCenter] addObserverForName:NSWorkspaceAccessibilityDisplayOptionsDidChangeNotification object:nil queue:[NSOperationQueue mainQueue] usingBlock:^(NSNotification *note){atomic_store(&reducedMotion,[NSWorkspace sharedWorkspace].accessibilityDisplayShouldReduceMotion);}];
   NSImage *image=[[NSImage alloc] initWithData:iconData];
   if (image) {
    image.size=NSMakeSize(18,18);
    [image setTemplate:YES];
    statusItem.button.image=image;
    statusItem.button.imagePosition=NSImageOnly;
    statusItem.button.title=@"";
   } else {
    statusItem.button.title=@"FT";
   }
  statusItem.button.toolTip=name;
  NSMenu *menu=[NSMenu new];
  NSMenuItem *show=[[NSMenuItem alloc] initWithTitle:openTitle action:@selector(show:) keyEquivalent:@""]; show.target=target;
  NSMenuItem *exit=[[NSMenuItem alloc] initWithTitle:quitTitle action:@selector(quit:) keyEquivalent:@""]; exit.target=target;
  [menu addItem:show]; [menu addItem:[NSMenuItem separatorItem]]; [menu addItem:exit]; statusItem.menu=menu;
 });
}
void freeturn_tray_close(void) {
  dispatch_async(dispatch_get_main_queue(), ^{ if(launchObserver) [[NSNotificationCenter defaultCenter] removeObserver:launchObserver]; launchObserver=nil; if(motionObserver) [[[NSWorkspace sharedWorkspace] notificationCenter] removeObserver:motionObserver];motionObserver=nil;if(watchingAppearance&&statusItem){[statusItem.button removeObserver:target forKeyPath:@"effectiveAppearance"];watchingAppearance=NO;}if(statusItem) [[NSStatusBar systemStatusBar] removeStatusItem:statusItem]; statusItem=nil; target=nil;darkStatusData=nil;lightStatusData=nil; });
}

void freeturn_tray_status(const unsigned char *dark,int darkLength,const unsigned char *light,int lightLength,const char *tooltip,int isTemplate) {
 @autoreleasepool {
 NSData *a=[NSData dataWithBytes:dark length:(NSUInteger)darkLength],*b=[NSData dataWithBytes:light length:(NSUInteger)lightLength];
 NSString *text=[NSString stringWithUTF8String:tooltip];
 dispatch_async(dispatch_get_main_queue(), ^{if(!statusItem)return;darkStatusData=a;lightStatusData=b;statusTemplate=isTemplate!=0;statusItem.button.toolTip=text;applyStatusIcon();});
 }
}
int freeturn_tray_reduced_motion(void) {return atomic_load(&reducedMotion)?1:0;}
void freeturn_tray_language(const char *open, const char *quit) {
 NSString *a=[NSString stringWithUTF8String:open],*b=[NSString stringWithUTF8String:quit];
 dispatch_async(dispatch_get_main_queue(), ^{ if(statusItem) { [statusItem.menu itemAtIndex:0].title=a; [statusItem.menu itemAtIndex:2].title=b; } });
}
