package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// prepareTemplate returns a filesystem that contains base/, postgres/, ...
// either from a local directory or downloaded from GitHub.
func prepareTemplate(o *Options) (fs.FS, func(), error) {
	noop := func() {}
	if o.TemplateDir != "" {
		if st, err := os.Stat(o.TemplateDir); err != nil || !st.IsDir() {
			return nil, noop, fmt.Errorf("--template-dir %q does not exist or is not a directory", o.TemplateDir)
		}
		return os.DirFS(o.TemplateDir), noop, nil
	}

	if strings.HasPrefix(o.Repo, "YOUR_GITHUB_USER/") {
		return nil, noop, errors.New("template repository is not configured: pass --repo owner/name " +
			"or build the CLI with -ldflags \"-X main.defaultRepo=owner/name\"")
	}

	tmp, err := os.MkdirTemp("", "golang-starter-*")
	if err != nil {
		return nil, noop, err
	}
	cleanup := func() { _ = os.RemoveAll(tmp) }

	fmt.Printf("\n→ downloading template github.com/%s@%s…\n", o.Repo, o.Ref)
	if err := downloadTemplate(o.Repo, o.Ref, tmp); err != nil {
		cleanup()
		return nil, noop, err
	}
	if st, err := os.Stat(filepath.Join(tmp, "base")); err != nil || !st.IsDir() {
		cleanup()
		return nil, noop, fmt.Errorf("%s@%s has no template/base", o.Repo, o.Ref)
	}
	return os.DirFS(tmp), cleanup, nil
}

func downloadTemplate(repo, ref, dst string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/tarball/%s", repo, ref)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "golang-starter")
	// Needed for private repositories and avoids the anonymous rate limit.
	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := (&http.Client{Timeout: 2 * time.Minute}).Do(req)
	if err != nil {
		return fmt.Errorf("downloading template: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return fmt.Errorf("repository or ref not found (%s@%s); set GITHUB_TOKEN for private repositories", repo, ref)
	default:
		return fmt.Errorf("github returned %s", resp.Status)
	}
	return extractTemplate(resp.Body, dst)
}

// extractTemplate unpacks the `template/` subtree of a GitHub tarball into dst.
// GitHub wraps everything in a single top-level "<owner>-<repo>-<sha>/" directory.
func extractTemplate(r io.Reader, dst string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("template archive: %w", err)
	}
	defer gz.Close()

	dst = filepath.Clean(dst)
	tr := tar.NewReader(gz)
	found := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("template archive: %w", err)
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeDir {
			continue // skips pax headers and symlinks
		}

		_, rel, ok := strings.Cut(hdr.Name, "/")
		if !ok || !strings.HasPrefix(rel, "template/") {
			continue
		}
		rel = strings.TrimPrefix(rel, "template/")
		if rel == "" {
			continue
		}

		target := filepath.Join(dst, filepath.FromSlash(rel))
		if target != dst && !strings.HasPrefix(target, dst+string(os.PathSeparator)) {
			return fmt.Errorf("unsafe path in archive: %q", hdr.Name)
		}

		if hdr.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fs.FileMode(hdr.Mode).Perm()|0o600)
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, tr); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
		found = true
	}
	if !found {
		return errors.New("the repository has no template/ directory")
	}
	return nil
}
