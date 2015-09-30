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

package build

import (
	"fmt"
	"sort"
	"strings"

	"github.com/fsouza/go-dockerclient"
)

// State is the build state
// TODO: document
type State struct {
	Config         docker.Config
	HostConfig     docker.HostConfig
	ImageID        string
	ParentID       string
	ContainerID    string
	ExportsID      string
	Commits        []string
	NoBaseImage    bool
	ProducedImage  bool
	CmdSet         bool
	CacheBusted    bool
	InjectCommands []string
	Dockerignore   []string
}

// NewState makes a fresh state
func NewState(b *Build) State {
	return State{
		Dockerignore: b.cfg.Dockerignore,
	}
}

// Commit adds a commit to the current state
func (s *State) Commit(msg string, args ...interface{}) *State {
	s.Commits = append(s.Commits, fmt.Sprintf(msg, args...))
	sort.Strings(s.Commits)
	return s
}

// GetCommits returns merged commits string
func (s State) GetCommits() string {
	return strings.Join(s.Commits, "; ")
}

// Equals returns true if the two states are equal
// NOTE: we identify unique commands by commits, so state uniqueness is simply a commit
func (s State) Equals(s2 State) bool {
	// TODO: compare other properties?
	return s.GetCommits() == s2.GetCommits()
}
