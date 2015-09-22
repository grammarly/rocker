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

package build2

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlan_Basic(t *testing.T) {
	p := makePlan(t, `
FROM ubuntu
`)

	expected := []Command{
		&CommandFrom{},
		&CommandCleanup{},
	}

	assert.Len(t, p, len(expected))
	for i, c := range expected {
		assert.IsType(t, c, p[i])
	}
}

func TestPlan_Run(t *testing.T) {
	p := makePlan(t, `
FROM ubuntu
RUN apt-get update
`)

	expected := []Command{
		&CommandFrom{},
		&CommandRun{},
		&CommandCommit{},
		&CommandCleanup{},
	}

	assert.Len(t, p, len(expected))
	for i, c := range expected {
		assert.IsType(t, c, p[i])
	}
}

func TestPlan_EnvRun(t *testing.T) {
	p := makePlan(t, `
FROM ubuntu
ENV name=web
ENV version=1.2
RUN apt-get update
`)

	expected := []Command{
		&CommandFrom{},
		&CommandEnv{},
		&CommandEnv{},
		&CommandCommit{},
		&CommandRun{},
		&CommandCommit{},
		&CommandCleanup{},
	}

	assert.Len(t, p, len(expected))
	for i, c := range expected {
		assert.IsType(t, c, p[i])
	}
}

func TestPlan_EnvLast(t *testing.T) {
	p := makePlan(t, `
FROM ubuntu
ENV name=web
`)

	expected := []Command{
		&CommandFrom{},
		&CommandEnv{},
		&CommandCommit{},
		&CommandCleanup{},
	}

	assert.Len(t, p, len(expected))
	for i, c := range expected {
		assert.IsType(t, c, p[i])
	}
}

func TestPlan_TwoFroms(t *testing.T) {
	p := makePlan(t, `
FROM ubuntu
FROM alpine
`)

	expected := []Command{
		&CommandFrom{},
		&CommandCleanup{},
		&CommandFrom{},
		&CommandCleanup{},
	}

	assert.Len(t, p, len(expected))
	for i, c := range expected {
		assert.IsType(t, c, p[i])
	}
}

func TestPlan_TwoFromsEnvBetween(t *testing.T) {
	p := makePlan(t, `
FROM ubuntu
ENV name=web
FROM alpine
`)

	expected := []Command{
		&CommandFrom{},
		&CommandEnv{},
		&CommandCommit{},
		&CommandCleanup{},
		&CommandFrom{},
		&CommandCleanup{},
	}

	assert.Len(t, p, len(expected))
	for i, c := range expected {
		assert.IsType(t, c, p[i])
	}
}

func TestPlan_TwoFromsTwoEnvs(t *testing.T) {
	p := makePlan(t, `
FROM ubuntu
ENV mode=build
FROM alpine
ENV mode=run
`)

	expected := []Command{
		&CommandFrom{},
		&CommandEnv{},
		&CommandCommit{},
		&CommandCleanup{},
		&CommandFrom{},
		&CommandEnv{},
		&CommandCommit{},
		&CommandCleanup{},
	}

	assert.Len(t, p, len(expected))
	for i, c := range expected {
		assert.IsType(t, c, p[i])
	}
}

func TestPlan_TagAtTheEnd(t *testing.T) {
	p := makePlan(t, `
FROM ubuntu
TAG my-build
`)

	expected := []Command{
		&CommandFrom{},
		&CommandTag{},
		&CommandCleanup{},
	}

	assert.Len(t, p, len(expected))
	for i, c := range expected {
		assert.IsType(t, c, p[i])
	}
}

func TestPlan_EnvBeforeTag(t *testing.T) {
	p := makePlan(t, `
FROM ubuntu
ENV type=web
TAG my-build
`)

	expected := []Command{
		&CommandFrom{},
		&CommandEnv{},
		&CommandCommit{},
		&CommandTag{},
		&CommandCleanup{},
	}

	assert.Len(t, p, len(expected))
	for i, c := range expected {
		assert.IsType(t, c, p[i])
	}
}

func TestPlan_TagInTheMiddle(t *testing.T) {
	p := makePlan(t, `
FROM ubuntu
TAG my-build
ENV type=web
`)

	expected := []Command{
		&CommandFrom{},
		&CommandTag{},
		&CommandEnv{},
		&CommandCommit{},
		&CommandCleanup{},
	}

	assert.Len(t, p, len(expected))
	for i, c := range expected {
		assert.IsType(t, c, p[i])
	}
}

func TestPlan_TagBeforeFrom(t *testing.T) {
	p := makePlan(t, `
FROM ubuntu
TAG my-build
FROM alpine
`)

	expected := []Command{
		&CommandFrom{},
		&CommandTag{},
		&CommandCleanup{},
		&CommandFrom{},
		&CommandCleanup{},
	}

	assert.Len(t, p, len(expected))
	for i, c := range expected {
		assert.IsType(t, c, p[i])
	}
}

func TestPlan_RunBeforeTag(t *testing.T) {
	p := makePlan(t, `
FROM ubuntu
RUN apt-get update
TAG my-build
`)

	expected := []Command{
		&CommandFrom{},
		&CommandRun{},
		&CommandCommit{},
		&CommandTag{},
		&CommandCleanup{},
	}

	assert.Len(t, p, len(expected))
	for i, c := range expected {
		assert.IsType(t, c, p[i])
	}
}

func TestPlan_Scratch(t *testing.T) {
	p := makePlan(t, `
FROM scratch
COPY rootfs /
`)

	expected := []Command{
		&CommandFrom{},
		&CommandCopy{},
		&CommandCommit{},
		&CommandCleanup{},
	}

	assert.Len(t, p, len(expected))
	for i, c := range expected {
		assert.IsType(t, c, p[i])
	}
}

func TestPlan_CleanupTaggedFinal(t *testing.T) {
	p := makePlan(t, `
FROM ubuntu
TAG dev
`)

	// from, tag, cleanup
	c := p[2]

	assert.IsType(t, &CommandCleanup{}, c)
	assert.True(t, c.(*CommandCleanup).tagged)
	assert.True(t, c.(*CommandCleanup).final)
}

func TestPlan_CleanupNotTaggedFinal(t *testing.T) {
	p := makePlan(t, `
FROM ubuntu
ENV foo=bar
`)

	// from, env, commit, cleanup
	c := p[3]

	assert.IsType(t, &CommandCleanup{}, c)
	assert.False(t, c.(*CommandCleanup).tagged)
	assert.True(t, c.(*CommandCleanup).final)
}

func TestPlan_CleanupNotTaggedMiddleFrom(t *testing.T) {
	p := makePlan(t, `
FROM ubuntu
ENV foo=bar
FROM alpine
`)

	// from, env, commit, cleanup, from, cleanup
	c := p[3]

	assert.IsType(t, &CommandCleanup{}, c)
	assert.False(t, c.(*CommandCleanup).tagged)
	assert.False(t, c.(*CommandCleanup).final)
}

// internal helpers

func makePlan(t *testing.T, rockerfileContent string) Plan {
	b, _ := makeBuild(t, rockerfileContent, Config{})

	p, err := NewPlan(b)
	if err != nil {
		t.Fatal(err)
	}

	return p
}
