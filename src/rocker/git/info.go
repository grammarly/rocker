/*-
 * Copyright 2015 Grammarly, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package git provides utilities of working with the current .git repo
package git

import (
	"fmt"
	"rocker/util"
	"strings"
)

// InfoData is the data structure that describes info taken from the current .git repo
type InfoData struct {
	Sha      string
	Branch   string
	RepoName string
	Remote   string
	URL      string
	Message  string
	Author   string
}

// ErrNotGitRepo is raised when given directory is not a .git repo
type ErrNotGitRepo struct {
	Cmd       *util.Cmd
	Directory string
}

// Error returns printable error string
func (err ErrNotGitRepo) Error() string {
	return fmt.Sprintf("Given directory is not a git repo: %s, tried to do %s", err.Directory, err.Cmd)
}

// Info gathers info from a given directory's .git repo
func Info(dir string) (gitInfo InfoData, err error) {
	if gitInfo.Sha, err = doGitCmd(dir, []string{"rev-parse", "HEAD"}); err != nil {
		return
	}

	if gitInfo.Message, err = doGitCmd(dir, []string{"log", "-1", "--pretty=%B"}); err != nil {
		return
	}
	if gitInfo.Author, err = doGitCmd(dir, []string{"log", "-1", "--pretty=%an <%ae>"}); err != nil {
		return
	}

	if gitInfo.Branch, err = doGitCmd(dir, []string{"rev-parse", "--abbrev-ref", "HEAD"}); err != nil {
		return
	}
	if gitInfo.Branch == "" {
		return
	}

	// ignore git errors of getting remote - it could not be set for current branch
	if gitInfo.Remote, _ = doGitCmd(dir, []string{"config", fmt.Sprintf("branch.%s.remote", gitInfo.Branch)}); gitInfo.Remote == "" {
		return gitInfo, nil
	}

	if gitInfo.URL, err = doGitCmd(dir, []string{"config", fmt.Sprintf("remote.%s.url", gitInfo.Remote)}); err != nil {
		return gitInfo, err
	}

	return gitInfo, nil
}

func doGitCmd(dir string, args []string) (out string, err error) {
	cmd := &util.Cmd{
		Args: append([]string{"/usr/bin/git"}, args...),
		Dir:  dir,
	}

	out, exitCode, err := util.ExecPipe(cmd)
	if err != nil {
		return "", err
	}
	if exitCode != 0 {
		if strings.Contains(out, "Not a git repository") {
			return "", &ErrNotGitRepo{cmd, dir}
		}
		return "", fmt.Errorf("Cmd `%s` exited with code: %d, output: %s", cmd, exitCode, out)
	}

	return strings.Trim(out, "\n"), nil
}
