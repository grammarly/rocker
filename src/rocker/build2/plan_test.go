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
		&CommandReset{},
		&CommandFrom{},
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
		&CommandReset{},
		&CommandFrom{},
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
		&CommandReset{},
		&CommandFrom{},
		&CommandEnv{},
		&CommandCommit{},
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
		&CommandReset{},
		&CommandFrom{},
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
	}

	assert.Len(t, p, len(expected))
	for i, c := range expected {
		assert.IsType(t, c, p[i])
	}
}

// internal helpers

func makePlan(t *testing.T, rockerfileContent string) Plan {
	b, _ := makeBuild(t, rockerfileContent, BuildConfig{})

	p, err := NewPlan(b)
	if err != nil {
		t.Fatal(err)
	}

	return p
}
