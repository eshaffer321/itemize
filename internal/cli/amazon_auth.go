package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"

	amazon "github.com/eshaffer321/amazon-go"
	"github.com/eshaffer321/itemize/internal/infrastructure/config"
)

const amazonOrdersURL = "https://www.amazon.com/gp/css/order-history?disableCsd=no-js"

// AmazonImportOptions controls Amazon browser-profile cookie import.
type AmazonImportOptions struct {
	ProfileDir     string
	Account        string
	CookieFile     string
	PlaywrightRoot string
	Headless       bool
	SkipAuthCheck  bool
	Out            io.Writer
}

// PrepareAmazonSetup creates the persistent browser profile used for an Amazon
// account. Keeping this convention inside itemize lets first-time users avoid
// choosing or managing Chromium profile paths themselves.
func PrepareAmazonSetup(account string) (string, error) {
	if account == "" {
		return "", fmt.Errorf("amazon setup requires -account <name>")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(account) {
		return "", fmt.Errorf("invalid Amazon account name %q: use only letters, numbers, dashes, and underscores", account)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	profileDir := filepath.Join(homeDir, ".itemize", "amazon", account)
	if err := os.MkdirAll(profileDir, 0700); err != nil {
		return "", fmt.Errorf("failed to create Amazon browser profile %q: %w", profileDir, err)
	}
	// #nosec G302 -- profileDir is a directory containing browser state; owner-only
	// traversal is intentional and more restrictive than the directory default.
	if err := os.Chmod(profileDir, 0700); err != nil {
		return "", fmt.Errorf("failed to secure Amazon browser profile %q: %w", profileDir, err)
	}
	return profileDir, nil
}

// RunAmazonSetup provides the guided first-time authentication path while the
// lower-level browser profile import remains available for advanced use.
func RunAmazonSetup(cfg *config.Config, opts AmazonImportOptions) error {
	profileDir, err := PrepareAmazonSetup(opts.Account)
	if err != nil {
		return err
	}
	opts.ProfileDir = profileDir
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if _, err := fmt.Fprintf(opts.Out, "Amazon setup for account %q.\nOpening Chromium; sign in to Amazon when prompted. Itemize will continue after Your Orders loads.\nBrowser profile: %s\n\n", opts.Account, profileDir); err != nil {
		return fmt.Errorf("failed to write setup instructions: %w", err)
	}
	if err := RunAmazonBrowserProfileImport(cfg, opts); err != nil {
		return err
	}
	_, err = fmt.Fprintf(opts.Out, "\nSetup complete. Test safely with:\n  itemize amazon -account %s -dry-run -days 14 -max 1\n", opts.Account)
	return err
}

// RunAmazonBrowserProfileImport imports Amazon cookies into itemize's Amazon cookie store.
func RunAmazonBrowserProfileImport(cfg *config.Config, opts AmazonImportOptions) error {
	if opts.ProfileDir == "" {
		return fmt.Errorf("-import-browser-profile requires a profile directory")
	}
	if cfg != nil && opts.CookieFile == "" {
		opts.CookieFile = cfg.Providers.Amazon.CookieFile
	}
	if opts.Out == nil {
		opts.Out = os.Stdout
	}

	resolvedProfileDir, err := expandUserPath(opts.ProfileDir)
	if err != nil {
		return fmt.Errorf("failed to resolve profile dir: %w", err)
	}
	if info, err := os.Stat(resolvedProfileDir); err != nil || !info.IsDir() {
		if err == nil {
			err = fmt.Errorf("not a directory")
		}
		return fmt.Errorf("invalid profile dir %q: %w", resolvedProfileDir, err)
	}
	if err := cleanStaleChromiumSingletons(resolvedProfileDir); err != nil {
		return err
	}

	root, err := resolvePlaywrightRoot(opts.PlaywrightRoot)
	if err != nil {
		return err
	}

	cookies, title, err := exportAmazonCookiesWithPlaywright(root, resolvedProfileDir, opts.Headless)
	if err != nil {
		return fmt.Errorf("failed to export cookies from browser profile: %w", explainAmazonCookieExportError(err))
	}
	if len(cookies) == 0 {
		return fmt.Errorf("no Amazon cookies were available in the browser profile")
	}

	if err := saveImportedAmazonCookies(cookies, opts); err != nil {
		var authErr *amazonImportAuthCheckError
		if errors.As(err, &authErr) {
			return fmt.Errorf("exported %d cookies from %q, but auth check failed: %w", len(cookies), resolvedProfileDir, authErr.err)
		}
		return err
	}

	destination := opts.CookieFile
	if destination == "" && opts.Account != "" {
		destination = "account " + opts.Account
	}
	if destination == "" {
		destination = "the default Amazon account"
	}
	if _, err := fmt.Fprintf(opts.Out, "Imported %d Amazon cookies from %q (%s) into %s.\n", len(cookies), resolvedProfileDir, title, destination); err != nil {
		return fmt.Errorf("failed to write import result: %w", err)
	}
	if !opts.SkipAuthCheck {
		if _, err := fmt.Fprintln(opts.Out, "Auth check passed."); err != nil {
			return fmt.Errorf("failed to write auth check result: %w", err)
		}
	}
	return nil
}

type amazonImportAuthCheckError struct {
	err error
}

func (e *amazonImportAuthCheckError) Error() string {
	return e.err.Error()
}

func (e *amazonImportAuthCheckError) Unwrap() error {
	return e.err
}

func saveImportedAmazonCookies(cookies []*amazon.Cookie, opts AmazonImportOptions) error {
	if !opts.SkipAuthCheck {
		if err := validateImportedAmazonCookies(cookies); err != nil {
			return &amazonImportAuthCheckError{err: err}
		}
	}

	client, err := newAmazonClient(opts.CookieFile, opts.Account)
	if err != nil {
		return fmt.Errorf("failed to create Amazon client: %w", err)
	}
	for _, c := range cookies {
		client.CookieStore().Set(c)
	}
	if err := client.SaveCookies(); err != nil {
		return fmt.Errorf("failed to save cookies: %w", err)
	}
	return nil
}

func validateImportedAmazonCookies(cookies []*amazon.Cookie) error {
	tempFile, err := os.CreateTemp("", "itemize-amazon-auth-*.json")
	if err != nil {
		return fmt.Errorf("failed to create temporary Amazon auth file: %w", err)
	}
	tempPath := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to close temporary Amazon auth file: %w", err)
	}
	defer func() { _ = os.Remove(tempPath) }()
	if err := os.Remove(tempPath); err != nil {
		return fmt.Errorf("failed to prepare temporary Amazon auth path: %w", err)
	}

	client, err := newAmazonClient(tempPath, "")
	if err != nil {
		return fmt.Errorf("failed to create Amazon validation client: %w", err)
	}
	for _, c := range cookies {
		client.CookieStore().Set(c)
	}
	return client.HealthCheck()
}

type exportedAmazonCookies struct {
	Title   string                  `json:"title"`
	Cookies []exportedBrowserCookie `json:"cookies"`
}

type exportedBrowserCookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	Secure   bool    `json:"secure"`
	HttpOnly bool    `json:"httpOnly"`
}

func exportAmazonCookiesWithPlaywright(playwrightRoot, profileDir string, headless bool) ([]*amazon.Cookie, string, error) {
	scriptPath, err := writeAmazonCookieExportScript(playwrightRoot)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = os.Remove(scriptPath) }()

	// #nosec G204 -- exec.Command does not invoke a shell; arguments are passed
	// directly to Node. scriptPath is a temporary file we just wrote, and the
	// profile path is data consumed by the script.
	cmd := exec.Command("node", scriptPath, profileDir, amazonOrdersURL, fmt.Sprintf("%t", headless))
	cmd.Dir = playwrightRoot
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, "", fmt.Errorf("node/playwright failed: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, "", err
	}

	var exported exportedAmazonCookies
	if err := json.Unmarshal(out, &exported); err != nil {
		return nil, "", fmt.Errorf("failed to parse Playwright cookie export: %w", err)
	}

	cookies := make([]*amazon.Cookie, 0, len(exported.Cookies))
	for _, c := range exported.Cookies {
		if c.Name == "" || c.Value == "" || !domainMatchesAmazon(c.Domain) {
			continue
		}
		cookies = append(cookies, &amazon.Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  int64(c.Expires),
			Secure:   c.Secure,
			HttpOnly: c.HttpOnly,
		})
	}

	return cookies, exported.Title, nil
}

func explainAmazonCookieExportError(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	lower := strings.ToLower(message)
	if strings.Contains(lower, "amazon sign-in") ||
		strings.Contains(lower, "profile did not open an authenticated amazon orders page") {
		return fmt.Errorf("browser profile is not logged into Amazon. Open the profile, sign into Amazon, then rerun without -headless so Amazon can refresh the session")
	}
	if strings.Contains(lower, "target page, context or browser has been closed") ||
		strings.Contains(lower, "sigtrap") {
		return fmt.Errorf("chromium closed before itemize could read Amazon cookies. Try again with -headless, or use a fresh Chromium/Playwright profile directory and sign into Amazon there")
	}
	return err
}

var chromiumProcessExists = processExists

func cleanStaleChromiumSingletons(profileDir string) error {
	lockPath := filepath.Join(profileDir, "SingletonLock")
	target, err := os.Readlink(lockPath)
	if err != nil {
		if os.IsNotExist(err) || err == syscall.EINVAL {
			return nil
		}
		return fmt.Errorf("failed to inspect Chromium profile lock: %w", err)
	}

	dash := strings.LastIndex(target, "-")
	if dash == -1 || dash == len(target)-1 {
		return nil
	}
	pid, err := strconv.Atoi(target[dash+1:])
	if err != nil {
		return nil
	}
	if chromiumProcessExists(pid) {
		return nil
	}

	for _, name := range []string{"SingletonLock", "SingletonSocket", "SingletonCookie"} {
		path := filepath.Join(profileDir, name)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove stale Chromium profile marker %q: %w", path, err)
		}
	}
	return nil
}

func processExists(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = process.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}

func writeAmazonCookieExportScript(playwrightRoot string) (string, error) {
	script := `const { chromium } = require("playwright");

async function main() {
  const profileDir = process.argv[2];
  const ordersURL = process.argv[3];
  const headless = process.argv[4] === "true";
  const options = {
    headless,
    viewport: { width: 1280, height: 800 },
    locale: "en-US"
  };
  if (headless) {
    const version = await chromiumVersion();
    options.userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/" + version + " Safari/537.36";
    options.extraHTTPHeaders = {
      "sec-ch-ua": "\"Chromium\";v=\"" + version.split(".")[0] + "\", \"Not A(Brand\";v=\"24\"",
      "sec-ch-ua-platform": "\"macOS\""
    };
  }
  const context = await chromium.launchPersistentContext(profileDir, options);
  if (headless) {
    await context.addInitScript(() => {
      Object.defineProperty(navigator, "webdriver", { get: () => undefined });
      Object.defineProperty(navigator, "languages", { get: () => ["en-US", "en"] });
      Object.defineProperty(navigator, "plugins", { get: () => [1, 2, 3, 4, 5] });
    });
  }
  try {
    const page = await context.newPage();
    await page.goto(ordersURL, { waitUntil: "domcontentloaded", timeout: 30000 });
    await page.waitForTimeout(2000);
    const state = await page.evaluate(() => {
      const body = document.body ? document.body.innerText : "";
      const login = !!document.querySelector("input#ap_email,input[name='email'],input#ap_password,input[name='password']") ||
        /amazon sign-in/i.test(document.title);
      const ready = !login && (body.includes("Your Orders") || document.querySelectorAll(".js-order-card,.order-card").length > 0);
      return { login, ready, title: document.title };
    });
    if (!state.ready) {
      if (headless) {
        throw new Error("profile did not open an authenticated Amazon orders page (title=" + JSON.stringify(state.title) + ", login=" + state.login + ")");
      }
      await page.waitForFunction(() => {
        const body = document.body ? document.body.innerText : "";
        const login = !!document.querySelector("input#ap_email,input[name='email'],input#ap_password,input[name='password']") ||
          /amazon sign-in/i.test(document.title);
        return !login && (body.includes("Your Orders") || document.querySelectorAll(".js-order-card,.order-card").length > 0);
      }, null, { timeout: 300000 }).catch(() => {
        throw new Error("Amazon sign-in did not complete within 5 minutes");
      });
    }
    const cookies = await context.cookies(["https://www.amazon.com", ordersURL]);
    const amazonCookies = cookies.filter(c => ["amazon.com", ".amazon.com", "www.amazon.com", ".www.amazon.com"].includes(c.domain));
    console.log(JSON.stringify({ title: await page.title(), cookies: amazonCookies }));
  } finally {
    await context.close();
  }
}

async function chromiumVersion() {
  const browser = await chromium.launch({ headless: true });
  try {
    return await browser.version();
  } finally {
    await browser.close();
  }
}

main().catch(err => {
  console.error(err && err.stack ? err.stack : String(err));
  process.exit(1);
});`

	f, err := os.CreateTemp(playwrightRoot, "itemize-amazon-import-*.cjs")
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	if _, err := f.WriteString(script); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

func resolvePlaywrightRoot(explicit string) (string, error) {
	var candidates []string
	if explicit != "" {
		candidates = append(candidates, explicit)
	}

	if cwd, err := os.Getwd(); err == nil {
		candidates = append(candidates, cwd)
		for dir := cwd; ; {
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			candidates = append(candidates, parent)
			dir = parent
		}
	}

	if root, err := npmRoot("-g"); err == nil && root != "" {
		candidates = append(candidates, filepath.Dir(root), root)
	}

	if home, err := os.UserHomeDir(); err == nil {
		matches, _ := filepath.Glob(filepath.Join(home, ".npm", "_npx", "*", "node_modules", "playwright", "package.json"))
		sort.Strings(matches)
		for i := len(matches) - 1; i >= 0; i-- {
			candidates = append(candidates, filepath.Dir(filepath.Dir(filepath.Dir(matches[i]))))
		}
	}

	seen := make(map[string]bool)
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		candidate, _ = expandUserPath(candidate)
		if seen[candidate] {
			continue
		}
		seen[candidate] = true
		if hasPlaywright(candidate) {
			return candidate, nil
		}
		if filepath.Base(candidate) == "playwright" && fileExists(filepath.Join(candidate, "package.json")) {
			return filepath.Dir(filepath.Dir(candidate)), nil
		}
	}

	return "", fmt.Errorf("playwright is required for Amazon login; install it with `npm install playwright`, then rerun setup (or pass -playwright-root pointing at a directory containing node_modules/playwright)")
}

func npmRoot(args ...string) (string, error) {
	cmdArgs := append([]string{"root"}, args...)
	// #nosec G204 -- args are fixed internal values ("root", optionally "-g")
	// used to locate Playwright; no shell is invoked.
	out, err := exec.Command("npm", cmdArgs...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func hasPlaywright(root string) bool {
	return fileExists(filepath.Join(root, "node_modules", "playwright", "package.json"))
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func newAmazonClient(cookieFile, account string) (*amazon.Client, error) {
	opts := []amazon.Option{amazon.WithAutoSave(false)}
	if cookieFile != "" {
		opts = append(opts, amazon.WithCookieFile(cookieFile))
	}
	if account != "" {
		opts = append(opts, amazon.WithAccount(account))
	}
	return amazon.NewClient(opts...)
}

func expandUserPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		path = filepath.Join(home, path[2:])
	}
	return filepath.Abs(path)
}

func domainMatchesAmazon(domain string) bool {
	return domain == "amazon.com" || domain == ".amazon.com" ||
		domain == "www.amazon.com" || domain == ".www.amazon.com"
}
