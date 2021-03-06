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
	"path"
	"regexp"
)

var bitbucketPattern = regexp.MustCompile(`^bitbucket\.org/(?P<owner>[a-z0-9A-Z_.\-]+)/(?P<repo>[a-z0-9A-Z_.\-]+)(?P<dir>/[a-z0-9A-Z_.\-/]*)?$`)
var bitbucketEtagRe = regexp.MustCompile(`^(hg|git)-`)

func GetBitbucketPerson(client HttpClient, match map[string]string) (*Person, error) {
	var userInfo struct {
		Repositories []*struct {
			Name     string
			Language string
		}
	}
	if err := httpGetJSON(client, expand("https://api.bitbucket.org/1.0/users/{owner}", match), &userInfo); err != nil {
		return nil, err
	}

	p := &Person{}
	for _, repo := range userInfo.Repositories {
		if repo.Language != "go" {
			continue
		}
		p.Projects = append(p.Projects, "bitbucket.org/"+match["owner"]+repo.Name)
	}

	return p, nil
}

func getBitbucketDoc(client HttpClient, match map[string]string, savedEtag string) (*Package, error) {

	if m := bitbucketEtagRe.FindStringSubmatch(savedEtag); m != nil {
		match["vcs"] = m[1]
	} else {
		var repo struct {
			Scm string
		}
		if err := httpGetJSON(client, expand("https://api.bitbucket.org/1.0/repositories/{owner}/{repo}", match), &repo); err != nil {
			return nil, err
		}
		match["vcs"] = repo.Scm
	}

	starCount := -1
	var followers struct {
		Count int
	}
	if err := httpGetJSON(client, expand("https://api.bitbucket.org/1.0/repositories/{owner}/{repo}/followers", match), &followers); err == nil {
		starCount = followers.Count
	}

	tags := make(map[string]string)
	for _, nodeType := range []string{"branches", "tags"} {
		var nodes map[string]struct {
			Node string
		}
		if err := httpGetJSON(client, expand("https://api.bitbucket.org/1.0/repositories/{owner}/{repo}/{0}", match, nodeType), &nodes); err != nil {
			return nil, err
		}
		for t, n := range nodes {
			tags[t] = n.Node
		}
	}

	var err error
	match["tag"], match["commit"], err = bestTag(tags, defaultTags[match["vcs"]])
	if err != nil {
		return nil, err
	}

	etag := expand("{vcs}-{commit}", match)
	if etag == savedEtag {
		return nil, ErrNotModified
	}

	var directory struct {
		Files []struct {
			Path string
		}
	}

	if err := httpGetJSON(client, expand("https://api.bitbucket.org/1.0/repositories/{owner}/{repo}/src/{tag}{dir}/", match), &directory); err != nil {
		return nil, err
	}

	var files []*source
	for _, f := range directory.Files {
		_, name := path.Split(f.Path)
		if isDocFile(name) {
			files = append(files, &source{
				name:      name,
				browseURL: expand("https://bitbucket.org/{owner}/{repo}/src/{tag}/{0}", match, f.Path),
				rawURL:    expand("https://api.bitbucket.org/1.0/repositories/{owner}/{repo}/raw/{tag}/{0}", match, f.Path),
			})
		}
	}

	if err := fetchFiles(client, files, nil); err != nil {
		return nil, err
	}

	b := builder{
		pdoc: &Package{
			LineFmt:     "%s#cl-%d",
			ImportPath:  match["originalImportPath"],
			ProjectRoot: expand("bitbucket.org/{owner}/{repo}", match),
			ProjectName: match["repo"],
			ProjectURL:  expand("https://bitbucket.org/{owner}/{repo}/", match),
			BrowseURL:   expand("https://bitbucket.org/{owner}/{repo}/src/{tag}{dir}", match),
			Etag:        etag,
			VCS:         match["vcs"],
			StarCount:   starCount,
		},
	}

	return b.build(files)
}
