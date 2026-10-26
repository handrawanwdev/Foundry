package work

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/handrawanwdev/foundry/internal/core"
	"github.com/handrawanwdev/foundry/internal/modules/workflow"
)

// Visual verification workflow commands are automatically added to projects
// with UI. They form a dependency chain: start → enumerate → capture → review → fix → re-verify.

// HasUI reports whether the project has a UI design that needs visual verification.
func HasUI(p *core.Project) bool {
	return p != nil && p.UI != nil && p.UI.Layout != nil && len(p.UI.Layout.Pages) > 0
}

// VisualVerificationCommands generates platform-aware visual verification commands.
// Each platform (web, android, ios, desktop) gets its own capture strategy.
func VisualVerificationCommands(p *core.Project, existingCmds []core.Command) []core.Command {
	if !HasUI(p) {
		return nil
	}

	// Detect target platforms
	platforms := detectTargetPlatforms(p)
	if len(platforms) == 0 {
		platforms = []string{core.PlatformWeb} // fallback to web
	}

	var cmds []core.Command
	pages := p.UI.Layout.Pages

	// Build page list per platform
	pagesByPlatform := make(map[string][]core.LayoutPage)
	for _, pg := range pages {
		plat := pg.Platform
		if plat == "" {
			plat = core.PlatformWeb // default
		}
		pagesByPlatform[plat] = append(pagesByPlatform[plat], pg)
	}

	// CMD-START-APP: Start application(s) for each platform
	for _, platform := range platforms {
		startCmd := generateStartCommand(platform, pagesByPlatform[platform], existingCmds)
		if startCmd != nil {
			cmds = append(cmds, *startCmd)
		}
	}

	// CMD-ENUMERATE-ROUTES: Enumerate routes per platform
	for _, platform := range platforms {
		enumCmd := generateEnumerateCommand(platform, pagesByPlatform[platform])
		if enumCmd != nil {
			cmds = append(cmds, *enumCmd)
		}
	}

	// CMD-CAPTURE-SCREENSHOTS: Platform-specific capture strategies
	for _, platform := range platforms {
		captureCmd := generateCaptureCommand(platform, pagesByPlatform[platform])
		if captureCmd != nil {
			cmds = append(cmds, *captureCmd)
		}
	}

	// CMD-SUBMIT-VISUAL-REVIEWS: Submit all reviews (platform-agnostic)
	submitCmd := generateSubmitCommand(pages, platforms)
	cmds = append(cmds, submitCmd)

	// CMD-CREATE-VISUAL-FIXES: Generate fixes if needed
	fixCmd := generateFixCommand(pages)
	cmds = append(cmds, fixCmd)

	// CMD-RE-VISUAL-VERIFY: Re-verify after fixes
	reverifyCmd := generateReverifyCommand(pages)
	cmds = append(cmds, reverifyCmd)

	return cmds
}

// detectTargetPlatforms returns list of platforms from project targets or pages
func detectTargetPlatforms(p *core.Project) []string {
	platformSet := make(map[string]bool)

	// From project platform targets
	if p.Platform != nil {
		for _, target := range p.Platform.Targets {
			if target.Kind == core.PlatformWeb || target.Kind == core.PlatformAndroid ||
				target.Kind == core.PlatformIOS || target.Kind == core.PlatformDesktop {
				platformSet[target.Kind] = true
			}
		}
	}

	// From UI pages
	for _, pg := range p.UI.Layout.Pages {
		if pg.Platform != "" {
			platformSet[pg.Platform] = true
		}
	}

	// Convert to slice
	var platforms []string
	for plat := range platformSet {
		platforms = append(platforms, plat)
	}
	return platforms
}

// generateStartCommand generates CMD-START-APP-{platform} command
func generateStartCommand(platform string, pages []core.LayoutPage, existingCmds []core.Command) *core.Command {
	if len(pages) == 0 {
		return nil
	}

	cmdID := fmt.Sprintf("CMD-START-APP-%s", strings.ToUpper(platform))
	cmdName := fmt.Sprintf("Start %s application", platform)

	var instructions string
	switch platform {
	case core.PlatformWeb:
		instructions = `1. Start the web development server (npm run dev, yarn dev, etc.).
2. Wait for the server to be ready (check port listening or health endpoint).
3. Verify the application responds to HTTP requests.
4. Report the base URL (e.g., http://localhost:3000).

**Acceptance Criteria:**
- Server starts without errors
- Application responds within 30 seconds
- Base URL is documented

**Evidence:**
- Startup logs
- curl or browser test output`

	case core.PlatformAndroid:
		instructions = `1. Start Android emulator or connect physical device.
2. Build and install the APK: ./gradlew installDebug or adb install app-debug.apk.
3. Launch the app: adb shell am start -n <package>/<activity>.
4. Verify app is running: adb shell pidof <package>.

**Acceptance Criteria:**
- Emulator/device is connected
- APK installed successfully
- App is running (PID exists)

**Evidence:**
- adb devices output
- Installation logs
- App PID`

	case core.PlatformIOS:
		instructions = `1. Start iOS Simulator (xcrun simctl boot <device-udid>).
2. Build and install the app: xcodebuild or use Xcode.
3. Launch the app: xcrun simctl launch <device-udid> <bundle-id>.
4. Verify app is running: xcrun simctl listapps | grep <bundle-id>.

**Acceptance Criteria:**
- Simulator is booted
- App installed successfully
- App is running

**Evidence:**
- Simulator UDID
- Installation logs
- App status`

	case core.PlatformDesktop:
		instructions = `1. Build the desktop application (cargo build, go build, etc.).
2. Launch the application binary.
3. Verify the application window is open and responsive.
4. Report the application process ID.

**Acceptance Criteria:**
- Application builds without errors
- Application launches successfully
- Window is visible and responsive

**Evidence:**
- Build logs
- Process ID
- Window manager output`
	}

	return &core.Command{
		ID:           cmdID,
		Name:         cmdName,
		Objective:    fmt.Sprintf("Ensure %s application can be started and is ready for visual testing.", platform),
		Instructions: instructions,
		DependsOn:    implementationDependencies(existingCmds),
		Artifacts: []core.ArtifactRef{
			{ID: core.ArtifactDesignRules},
			{ID: core.ArtifactLayoutSpec},
		},
		Outputs:    []string{fmt.Sprintf("%s application running", platform)},
		Acceptance: []string{"Application starts without errors"},
		Status:     core.CommandDraft,
	}
}

// generateEnumerateCommand generates CMD-ENUMERATE-ROUTES-{platform} command
func generateEnumerateCommand(platform string, pages []core.LayoutPage) *core.Command {
	if len(pages) == 0 {
		return nil
	}

	cmdID := fmt.Sprintf("CMD-ENUMERATE-ROUTES-%s", strings.ToUpper(platform))
	cmdName := fmt.Sprintf("Enumerate %s routes/screens", platform)

	var pageList strings.Builder
	for _, pg := range pages {
		if platform == core.PlatformWeb {
			fmt.Fprintf(&pageList, "- %s (route: %s)\n", pg.ID, pg.Route)
		} else {
			fmt.Fprintf(&pageList, "- %s (screen: %s)\n", pg.ID, pg.Name)
		}
	}

	var instructions string
	switch platform {
	case core.PlatformWeb:
		instructions = fmt.Sprintf(`1. Inspect the web application's routing configuration.
2. List all accessible routes/pages.
3. Verify each route is accessible via HTTP GET.
4. Report any missing or broken routes.

**Pages to verify:**
%s
**Acceptance Criteria:**
- All routes are accessible
- No 404 errors
- Route list is complete

**Evidence:**
- Route list with status codes`, pageList.String())

	case core.PlatformAndroid, core.PlatformIOS:
		instructions = fmt.Sprintf(`1. Navigate through the app using UI automation or manual testing.
2. List all screens/pages that can be reached.
3. Verify each screen is accessible via navigation.
4. Report the navigation path to each screen.

**Screens to verify:**
%s
**Acceptance Criteria:**
- All screens are reachable
- Navigation paths are documented
- No crashes during navigation

**Evidence:**
- Navigation flow diagram
- Screen list with access paths`, pageList.String())

	case core.PlatformDesktop:
		instructions = fmt.Sprintf(`1. Explore the desktop application UI.
2. List all windows/screens accessible from main menu or navigation.
3. Verify each screen is accessible.
4. Report the navigation path to each screen.

**Screens to verify:**
%s
**Acceptance Criteria:**
- All screens are accessible
- No crashes during navigation

**Evidence:**
- Screen list
- Navigation paths`, pageList.String())
	}

	return &core.Command{
		ID:           cmdID,
		Name:         cmdName,
		Objective:    fmt.Sprintf("Discover all %s routes/screens and verify they match the layout specification.", platform),
		Instructions: instructions,
		DependsOn:    []string{fmt.Sprintf("CMD-START-APP-%s", strings.ToUpper(platform))},
		Artifacts: []core.ArtifactRef{
			{ID: core.ArtifactDesignRules},
			{ID: core.ArtifactLayoutSpec},
		},
		Outputs:    []string{"Complete route/screen list"},
		Acceptance: []string{"All routes/screens are accessible"},
		Status:     core.CommandDraft,
	}
}

// generateCaptureCommand generates CMD-CAPTURE-SCREENSHOTS-{platform} command with platform-specific strategies
func generateCaptureCommand(platform string, pages []core.LayoutPage) *core.Command {
	if len(pages) == 0 {
		return nil
	}

	cmdID := fmt.Sprintf("CMD-CAPTURE-SCREENSHOTS-%s", strings.ToUpper(platform))
	cmdName := fmt.Sprintf("Capture %s screenshots", platform)

	var pageList strings.Builder
	pageIDs := make([]string, 0, len(pages))
	for _, pg := range pages {
		fmt.Fprintf(&pageList, "- %s\n", pg.ID)
		pageIDs = append(pageIDs, pg.ID)
	}

	var instructions string
	switch platform {
	case core.PlatformWeb:
		instructions = fmt.Sprintf(`1. Use browser automation (Playwright, Puppeteer, or similar).
2. For each page, capture screenshots at multiple viewports:
   - Desktop: 1920x1080
   - Tablet: 768x1024
   - Mobile: 375x667
3. Capture DOM snapshot (accessibility tree) for each viewport.
4. Save screenshots to visual-review/<page-id>/<viewport>.png.
5. Save DOM snapshots to visual-review/<page-id>/<viewport>.json.

**Pages to capture:**
%s
**Acceptance Criteria:**
- Every page has screenshots at 3 viewports
- DOM snapshots captured with computed styles
- Files saved in correct locations

**Evidence:**
- Screenshot files
- DOM snapshot JSON files`, pageList.String())

	case core.PlatformAndroid:
		instructions = fmt.Sprintf(`1. Use adb to capture screenshots:
   adb exec-out screencap -p > visual-review/<page-id>/screenshot.png
2. Capture accessibility tree (UI hierarchy):
   adb shell uiautomator dump /sdcard/window_dump.xml
   adb pull /sdcard/window_dump.xml visual-review/<page-id>/ui-dump.xml
3. For different screen sizes, use emulator with different DPI/size configs.
4. Navigate to each screen and capture.

**Screens to capture:**
%s
**Acceptance Criteria:**
- Every screen has screenshot
- UI hierarchy captured
- Files saved in correct locations

**Evidence:**
- Screenshot PNG files
- UI dump XML files`, pageList.String())

	case core.PlatformIOS:
		instructions = fmt.Sprintf(`1. Use xcrun simctl to capture screenshots:
   xcrun simctl io <device-udid> screenshot visual-review/<page-id>/screenshot.png
2. Capture accessibility tree using Accessibility Inspector or UI test framework.
3. For different screen sizes, use different simulator devices (iPhone SE, iPhone 14 Pro Max, iPad).
4. Navigate to each screen and capture.

**Screens to capture:**
%s
**Acceptance Criteria:**
- Every screen has screenshot
- Accessibility data captured
- Files saved in correct locations

**Evidence:**
- Screenshot PNG files
- Accessibility data`, pageList.String())

	case core.PlatformDesktop:
		instructions = fmt.Sprintf(`1. Use OS-specific screenshot tools:
   - Linux: gnome-screenshot, scrot, or import (ImageMagick)
   - macOS: screencapture
   - Windows: snippingtool or PowerShell
   Example: screencapture visual-review/<page-id>/screenshot.png
2. Capture window hierarchy/UI tree if possible (platform-dependent).
3. For different window sizes, resize the application window.
4. Navigate to each screen/window and capture.

**Screens to capture:**
%s
**Acceptance Criteria:**
- Every screen has screenshot
- Different window sizes captured (if applicable)
- Files saved in correct locations

**Evidence:**
- Screenshot files`, pageList.String())
	}

	return &core.Command{
		ID:           cmdID,
		Name:         cmdName,
		Objective:    fmt.Sprintf("Capture visual evidence for every %s page/screen.", platform),
		Instructions: instructions,
		DependsOn:    []string{fmt.Sprintf("CMD-ENUMERATE-ROUTES-%s", strings.ToUpper(platform))},
		Artifacts: []core.ArtifactRef{
			{ID: core.ArtifactDesignRules},
			{ID: core.ArtifactLayoutSpec},
		},
		Outputs:    []string{"Screenshot files", "Accessibility/UI data"},
		Acceptance: []string{"Every page/screen captured"},
		Status:     core.CommandDraft,
	}
}

// generateSubmitCommand generates CMD-SUBMIT-VISUAL-REVIEWS command (platform-agnostic)
func generateSubmitCommand(pages []core.LayoutPage, platforms []string) core.Command {
	var pageList strings.Builder
	pageIDs := make([]string, 0, len(pages))
	for _, pg := range pages {
		fmt.Fprintf(&pageList, "- %s (platform: %s)\n", pg.ID, pg.Platform)
		pageIDs = append(pageIDs, pg.ID)
	}

	// Build dependencies on all capture commands
	var dependsOn []string
	for _, plat := range platforms {
		dependsOn = append(dependsOn, fmt.Sprintf("CMD-CAPTURE-SCREENSHOTS-%s", strings.ToUpper(plat)))
	}

	instructions := fmt.Sprintf(`1. For each page and platform, call Foundry's record_visual_review API.
2. Provide:
   - page: page ID
   - platform: platform name (web, android, ios, desktop)
   - screenshot: path to screenshot file
   - viewport/device: viewport size (web) or device name (mobile/desktop)
   - nodes: accessibility tree or DOM snapshot
   - state: "default", "loading", "empty", "error", etc.
3. Submit reviews for all pages and platforms.
4. Record the review IDs for tracking.

**API Call Example:**
POST /api/v1/project/visual/reviews
{
  "page": "dashboard",
  "platform": "android",
  "screenshot": "visual-review/dashboard/screenshot.png",
  "viewport": {"name": "pixel-5"},
  "nodes": [...],
  "state": "default"
}

**Pages to submit:**
%s
**Platforms:** %s

**Acceptance Criteria:**
- All pages submitted for all platforms
- Review IDs recorded
- No API errors

**Evidence:**
- API call logs
- Review IDs`, pageList.String(), strings.Join(platforms, ", "))

	return core.Command{
		ID:           "CMD-SUBMIT-VISUAL-REVIEWS",
		Name:         "Submit visual reviews to Foundry",
		Objective:    "Submit captured screenshots and accessibility data to Foundry for evaluation.",
		Instructions: instructions,
		DependsOn:    dependsOn,
		Artifacts: []core.ArtifactRef{
			{ID: core.ArtifactDesignRules},
			{ID: core.ArtifactLayoutSpec},
		},
		Outputs:    []string{"Visual review submissions", "Review IDs"},
		Acceptance: []string{"All reviews submitted successfully"},
		Status:     core.CommandDraft,
	}
}

// generateFixCommand generates CMD-CREATE-VISUAL-FIXES command
func generateFixCommand(pages []core.LayoutPage) core.Command {
	pageIDs := make([]string, 0, len(pages))
	for _, pg := range pages {
		pageIDs = append(pageIDs, pg.ID)
	}

	instructions := fmt.Sprintf(`1. Call Foundry's get_visual_review API to retrieve evaluation results.
2. Check if there are any findings (contrast, overflow, spacing, accessibility, etc.).
3. If findings exist:
   a. Call create_visual_fixes API to generate correction commands.
   b. Record the new command IDs.
4. If no findings, report that visual quality passed.

**Pages evaluated:** %s

**API Calls:**
GET /api/v1/project/visual/review
POST /api/v1/project/visual/fixes

**Acceptance Criteria:**
- Visual review results retrieved
- Fixes created if findings exist
- New command IDs recorded (if any)

**Evidence:**
- Visual review report
- List of findings (if any)
- New command IDs (if fixes created)`, strings.Join(pageIDs, ", "))

	return core.Command{
		ID:           "CMD-CREATE-VISUAL-FIXES",
		Name:         "Create visual fixes from findings",
		Objective:    "Generate correction commands for any visual issues found during evaluation.",
		Instructions: instructions,
		DependsOn:    []string{"CMD-SUBMIT-VISUAL-REVIEWS"},
		Artifacts: []core.ArtifactRef{
			{ID: core.ArtifactDesignRules},
			{ID: core.ArtifactLayoutSpec},
		},
		Outputs:    []string{"Visual review report", "Correction commands (if needed)"},
		Acceptance: []string{"Visual review results checked", "Fixes created if needed"},
		Status:     core.CommandDraft,
	}
}

// generateReverifyCommand generates CMD-RE-VISUAL-VERIFY command
func generateReverifyCommand(pages []core.LayoutPage) core.Command {
	pageIDs := make([]string, 0, len(pages))
	for _, pg := range pages {
		pageIDs = append(pageIDs, pg.ID)
	}

	instructions := fmt.Sprintf(`1. If visual fixes were created in CMD-CREATE-VISUAL-FIXES:
   a. Implement the fixes.
   b. Re-capture screenshots for affected pages (all platforms).
   c. Re-submit visual reviews.
   d. Verify all findings are resolved.
2. If no fixes were needed, skip this command.
3. Report final visual quality status.

**Pages to re-verify (if fixes were applied):** %s

**Acceptance Criteria:**
- All visual findings resolved (if any existed)
- Final visual review shows pass or partial status
- No critical or high severity findings remain

**Evidence:**
- Re-captured screenshots
- Final visual review report
- Resolution status`, strings.Join(pageIDs, ", "))

	return core.Command{
		ID:           "CMD-RE-VISUAL-VERIFY",
		Name:         "Re-verify visual quality after fixes",
		Objective:    "Re-capture and re-submit screenshots to verify visual fixes resolved all issues.",
		Instructions: instructions,
		DependsOn:    []string{"CMD-CREATE-VISUAL-FIXES"},
		Artifacts: []core.ArtifactRef{
			{ID: core.ArtifactDesignRules},
			{ID: core.ArtifactLayoutSpec},
		},
		Outputs:    []string{"Final visual verification report"},
		Acceptance: []string{"All visual findings resolved", "Visual quality verified"},
		Status:     core.CommandDraft,
	}
}

// implementationDependencies returns the IDs of commands that implement UI.
// Visual verification depends on all UI implementation commands.
func implementationDependencies(cmds []core.Command) []string {
	var deps []string
	for _, c := range cmds {
		// Commands that reference design artifacts are likely UI implementation
		for _, art := range c.Artifacts {
			if art.ID == core.ArtifactDesignRules || art.ID == core.ArtifactLayoutSpec {
				if !slices.Contains(deps, c.ID) {
					deps = append(deps, c.ID)
				}
				break
			}
		}
	}
	return deps
}

// VisualVerificationRules are added to the workflow prompt when the project has UI.
const VisualVerificationRules = `
VISUAL VERIFICATION RULES (for projects with UI):
- After implementing UI, the workflow MUST include visual verification steps.
- Visual verification is platform-aware: each target platform gets its own start, enumerate, and capture commands.
- Web pages use browser automation (Playwright/Puppeteer) with viewports (desktop 1920x1080, tablet 768x1024, mobile 375x667) and DOM snapshots.
- Android pages use adb (adb exec-out screencap -p for screenshots, uiautomator dump for accessibility tree).
- iOS pages use xcrun simctl (xcrun simctl io <udid> screenshot for screenshots).
- Desktop pages use OS screenshot tools (screencapture, gnome-screenshot, snippingtool) at multiple window sizes.
- Screenshots are required evidence; a page without screenshots is untested, not passed.
- Different states (default, loading, empty, error) must be captured where applicable.
- Visual findings (contrast, overflow, spacing, accessibility) must be fixed before finalizing.
- Build success, lint pass, and unit tests do NOT count as visual verification.
- If visual tooling is unavailable for a platform, report the limitation explicitly; do not claim visual quality is verified.
`

// appendVisualVerificationCommands adds visual verification commands to the workflow
// if the project has UI and doesn't already have visual verification commands.
// This is called explicitly when visual verification is needed, not automatically.
func appendVisualVerificationCommands(p *core.Project, docs []core.Artifact) error {
	if !HasUI(p) || p.Workflow == nil || len(p.Workflow.Commands) == 0 {
		return nil
	}

	// Check if visual verification commands already exist
	for _, cmd := range p.Workflow.Commands {
		if cmd.ID == "CMD-START-APP" || cmd.ID == "CMD-ENUMERATE-ROUTES" ||
			cmd.ID == "CMD-CAPTURE-SCREENSHOTS" || cmd.ID == "CMD-SUBMIT-VISUAL-REVIEWS" ||
			cmd.ID == "CMD-CREATE-VISUAL-FIXES" || cmd.ID == "CMD-RE-VISUAL-VERIFY" {
			// Visual verification commands already exist, skip
			return nil
		}
	}

	// Generate visual verification commands with proper page references
	visualCmds := VisualVerificationCommands(p, p.Workflow.Commands)
	if len(visualCmds) == 0 {
		return nil
	}

	// Append visual verification commands to workflow
	p.Workflow.Commands = append(p.Workflow.Commands, visualCmds...)
	p.Workflow.Version++

	// Re-validate the workflow
	hashes := workflow.Hashes(docs)
	if err := checkCommands(p.Workflow.Commands, hashes, p.Requirements); err != nil {
		return fmt.Errorf("visual verification commands invalid: %w", err)
	}

	return nil
}

// AddVisualVerificationCommands explicitly adds visual verification commands to the workflow.
// This should be called after the main workflow is complete and the project has UI.
// Returns the number of commands added, or an error if validation fails.
func AddVisualVerificationCommands(p *core.Project, docs []core.Artifact) (int, error) {
	if !HasUI(p) {
		return 0, nil
	}
	if p.Workflow == nil || len(p.Workflow.Commands) == 0 {
		return 0, errors.New("workflow belum memiliki command; selesaikan workflow utama terlebih dahulu")
	}

	// Check if visual verification commands already exist
	for _, cmd := range p.Workflow.Commands {
		if cmd.ID == "CMD-START-APP" {
			return 0, nil // Already added
		}
	}

	beforeCount := len(p.Workflow.Commands)
	if err := appendVisualVerificationCommands(p, docs); err != nil {
		return 0, err
	}

	return len(p.Workflow.Commands) - beforeCount, nil
}
