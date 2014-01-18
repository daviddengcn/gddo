// Copyright 2011 Gary Burd
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package doc

import (
	"errors"
	"net/url"
	"regexp"
	//"strconv"
	"strings"

	// "log"
)

var (
	googleRepoRe     = regexp.MustCompile(`id="checkoutcmd">(hg|git|svn)`)
	googleRevisionRe = regexp.MustCompile(`<h2>(?:[^ ]+ - )?Revision *([^:]+):`)
	googleEtagRe     = regexp.MustCompile(`^(hg|git|svn)-`)
	googleFileRe     = regexp.MustCompile(`<li><a href="([^"/]+)"`)
	googlePattern    = regexp.MustCompile(`^code\.google\.com/p/(?P<repo>[a-z0-9\-]+)(:?\.(?P<subrepo>[a-z0-9\-]+))?(?P<dir>/[a-z0-9A-Z_.\-/]+)?$`)

	// googleStarRe     = regexp.MustCompile(`<span\s+id="star_count">\s*([0-9])+\s*</span>`)
)

func getGoogleDoc(client HttpClient, match map[string]string, savedEtag string) (*Package, error) {
	setupGoogleMatch(match)
	if m := googleEtagRe.FindStringSubmatch(savedEtag); m != nil {
		match["vcs"] = m[1]
	} else if err := getGoogleVCS(client, match); err != nil {
		return nil, err
	}

	starCount := -1
	/*
		p, err := httpGetBytes(client, expand("https://code.google.com/p/{repo}/", match), nil)
		if err == nil {
			log.Printf("[getGoogleDoc] httpGetBytes %s: %s", expand("https://code.google.com/p/{repo}/", match), string(p))
			m := googleStarRe.FindSubmatch(p)
			if m != nil {
				cnt, err := strconv.Atoi(strings.TrimSpace(string(m[1])))
				if err == nil {
					starCount = cnt
				}
			}
		}
	*/

	// Scrape the repo browser to find the project revision and individual Go files.
	p, err := httpGetBytes(client, expand("http://{subrepo}{dot}{repo}.googlecode.com/{vcs}{dir}/", match), nil)
	if err != nil {
		return nil, err
	}

	var etag string
	if m := googleRevisionRe.FindSubmatch(p); m == nil {
		return nil, errors.New("Could not find revision for " + match["importPath"])
	} else {
		etag = expand("{vcs}-{0}", match, string(m[1]))
		if etag == savedEtag {
			return nil, ErrNotModified
		}
	}

	var files []*source
	for _, m := range googleFileRe.FindAllSubmatch(p, -1) {
		fname := string(m[1])
		if isDocFile(fname) {
			files = append(files, &source{
				name:      fname,
				browseURL: expand("http://code.google.com/p/{repo}/source/browse{dir}/{0}{query}", match, fname),
				rawURL:    expand("http://{subrepo}{dot}{repo}.googlecode.com/{vcs}{dir}/{0}", match, fname),
			})
		}
	}

	if err := fetchFiles(client, files, nil); err != nil {
		return nil, err
	}

	var projectURL string
	if match["subrepo"] == "" {
		projectURL = expand("https://code.google.com/p/{repo}/", match)
	} else {
		projectURL = expand("https://code.google.com/p/{repo}/source/browse?repo={subrepo}", match)
	}

	b := &builder{
		pdoc: &Package{
			LineFmt:     "%s#%d",
			ImportPath:  match["originalImportPath"],
			ProjectRoot: expand("code.google.com/p/{repo}{dot}{subrepo}", match),
			ProjectName: expand("{repo}{dot}{subrepo}", match),
			ProjectURL:  projectURL,
			BrowseURL:   expand("http://code.google.com/p/{repo}/source/browse{dir}/{query}", match),
			Etag:        etag,
			VCS:         match["vcs"],
			StarCount:   starCount,
		},
	}

	return b.build(files)
}

func setupGoogleMatch(match map[string]string) {
	if s := match["subrepo"]; s != "" {
		match["dot"] = "."
		match["query"] = "?repo=" + s
	} else {
		match["dot"] = ""
		match["query"] = ""
	}
}

func getGoogleVCS(client HttpClient, match map[string]string) error {
	// Scrape the HTML project page to find the VCS.
	p, err := httpGetBytes(client, expand("http://code.google.com/p/{repo}/source/checkout", match), nil)
	if err != nil {
		return err
	}
	m := googleRepoRe.FindSubmatch(p)
	if m == nil {
		return NotFoundError{"Could not VCS on Google Code project page."}
	}
	match["vcs"] = string(m[1])
	return nil
}

func getStandardDoc(client HttpClient, importPath string, savedEtag string) (*Package, error) {

	p, err := httpGetBytes(client, "http://go.googlecode.com/hg-history/release/src/pkg/"+importPath+"/", nil)
	if err != nil {
		return nil, err
	}

	var etag string
	if m := googleRevisionRe.FindSubmatch(p); m == nil {
		return nil, errors.New("Could not find revision for " + importPath)
	} else {
		etag = string(m[1])
		if etag == savedEtag {
			return nil, ErrNotModified
		}
	}

	var files []*source
	for _, m := range googleFileRe.FindAllSubmatch(p, -1) {
		fname := strings.Split(string(m[1]), "?")[0]
		if isDocFile(fname) {
			files = append(files, &source{
				name:      fname,
				browseURL: "http://code.google.com/p/go/source/browse/src/pkg/" + importPath + "/" + fname + "?name=release",
				rawURL:    "http://go.googlecode.com/hg-history/release/src/pkg/" + importPath + "/" + fname,
			})
		}
	}

	if err := fetchFiles(client, files, nil); err != nil {
		return nil, err
	}

	b := &builder{
		pdoc: &Package{
			LineFmt:     "%s#%d",
			ImportPath:  importPath,
			ProjectRoot: "",
			ProjectName: "Go",
			ProjectURL:  "https://code.google.com/p/go/",
			BrowseURL:   "http://code.google.com/p/go/source/browse/src/pkg/" + importPath + "?name=release",
			Etag:        etag,
			VCS:         "hg",
		},
	}

	return b.build(files)
}

func getGooglePresentation(client HttpClient, match map[string]string) (*Presentation, error) {
	setupGoogleMatch(match)
	if err := getGoogleVCS(client, match); err != nil {
		return nil, err
	}

	rawBase, err := url.Parse(expand("http://{subrepo}{dot}{repo}.googlecode.com/{vcs}{dir}/", match))
	if err != nil {
		return nil, err
	}

	p, err := httpGetBytes(client, expand("http://{subrepo}{dot}{repo}.googlecode.com/{vcs}{dir}/{file}", match), nil)
	if err != nil {
		return nil, err
	}

	b := &presBuilder{
		data:     p,
		filename: match["file"],
		fetch: func(files []*source) error {
			for _, f := range files {
				u, err := rawBase.Parse(f.name)
				if err != nil {
					return err
				}
				f.rawURL = u.String()
			}
			return fetchFiles(client, files, nil)
		},
		resolveURL: func(fname string) string {
			u, err := rawBase.Parse(fname)
			if err != nil {
				return "/notfound"
			}
			return u.String()
		},
	}

	return b.build()
}
