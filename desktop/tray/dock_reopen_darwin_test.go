//go:build darwin

package tray

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// Exercise the actual Objective-C installation and delegate callback without
// starting NSApplication, opening windows or changing a running app's delegate.
func TestDockReopenHandler(t *testing.T) {
	source, err := filepath.Abs("tray_darwin.m")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	harness := filepath.Join(dir, "reopen.m")
	program := `#import <Cocoa/Cocoa.h>
#include ` + strconv.Quote(source) + `
static int showCount;
void freeturn_tray_action(int action) { if(action==1) showCount++; }
@interface MissingReopen : NSObject @end
@implementation MissingReopen @end
@interface ExistingReopen : NSObject <NSApplicationDelegate> @end
@implementation ExistingReopen
- (BOOL)applicationShouldHandleReopen:(NSApplication *)app hasVisibleWindows:(BOOL)visible { return YES; }
@end
int main(void) {
 @autoreleasepool {
  installDockReopenHandler(nil);
  MissingReopen *delegate=[MissingReopen new];
  SEL selector=@selector(applicationShouldHandleReopen:hasVisibleWindows:);
  installDockReopenHandler(delegate);
  if(![delegate respondsToSelector:selector]) return 1;
  IMP first=class_getMethodImplementation([delegate class],selector);
  installDockReopenHandler(delegate);
  if(first!=class_getMethodImplementation([delegate class],selector)) return 2;
  id<NSApplicationDelegate> handler=(id<NSApplicationDelegate>)delegate;
  if([handler applicationShouldHandleReopen:nil hasVisibleWindows:NO]) return 3;
  if([handler applicationShouldHandleReopen:nil hasVisibleWindows:YES]) return 4;
  if(showCount!=2) return 5;
  ExistingReopen *existing=[ExistingReopen new];
  IMP original=class_getMethodImplementation([existing class],selector);
  installDockReopenHandler(existing);
  if(original!=class_getMethodImplementation([existing class],selector)) return 6;
  if(![existing applicationShouldHandleReopen:nil hasVisibleWindows:NO]) return 7;
  if(showCount!=2) return 8;
 }
 return 0;
}
`
	if err := os.WriteFile(harness, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	binary := filepath.Join(dir, "reopen")
	if out, err := exec.CommandContext(ctx, "/usr/bin/clang", "-fobjc-arc", "-framework", "Cocoa", harness, "-o", binary).CombinedOutput(); err != nil {
		t.Fatalf("compile native reopen test: %v\n%s", err, out)
	}
	if out, err := exec.CommandContext(ctx, binary).CombinedOutput(); err != nil {
		t.Fatalf("native reopen callback: %v\n%s", err, out)
	}
}
