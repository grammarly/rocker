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

import "strings"

type Plan []Command

func NewPlan(commands []ConfigCommand, finalCleanup bool) (plan Plan, err error) {
	plan = Plan{}

	committed := true

	commit := func() {
		plan = append(plan, &CommandCommit{})
		committed = true
	}

	cleanup := func(i int) {
		plan = append(plan, &CommandCleanup{
			final:  i == len(commands)-1,
			tagged: strings.Contains("tag push from", commands[i].name),
		})
	}

	alwaysCommitBefore := "run attach add copy tag push export import"
	alwaysCommitAfter := "run attach add copy export import"
	neverCommitAfter := "from maintainer tag push"

	for i := 0; i < len(commands); i++ {
		cfg := commands[i]

		cmd, err := NewCommand(cfg)
		if err != nil {
			return nil, err
		}

		// We want to reset the collected state between FROM instructions
		// But do it only if it's not the first FROM
		if cfg.name == "from" {
			if !committed {
				commit()
			}
			if i > 0 {
				cleanup(i - 1)
			}
		}

		// Commit before commands that require state
		if strings.Contains(alwaysCommitBefore, cfg.name) && !committed {
			commit()
		}

		plan = append(plan, cmd)

		// Some commands need immediate commit
		if strings.Contains(alwaysCommitAfter, cfg.name) {
			commit()
		} else if !strings.Contains(neverCommitAfter, cfg.name) {
			// Reset the committed state for the rest of commands and
			// start collecting them
			committed = false

			// If we reached the end of Rockerfile, do the final commit
			// As you noticed, the final commit will not happen if the last
			// command was TAG, PUSH or FROM
			if i == len(commands)-1 {
				commit()
			}
		}

		// Always cleanup at the end
		if i == len(commands)-1 && finalCleanup {
			cleanup(i)
		}
	}

	return plan, err
}
