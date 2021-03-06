/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package branchcleaner

import (
	"fmt"
	"testing"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/driver/fake"
	"github.com/jenkins-x/lighthouse/pkg/scmprovider"
	"github.com/sirupsen/logrus"
)

func TestBranchCleaner(t *testing.T) {
	baseRepoOrg := "my-org"
	baseRepoRepo := "repo"
	baseRepoFullName := fmt.Sprintf("%s/%s", baseRepoOrg, baseRepoRepo)

	testcases := []struct {
		name                 string
		prAction             scm.Action
		merged               bool
		headRepoFullName     string
		branchDeleteExpected bool
	}{
		{
			name:                 "Opened PR nothing to do",
			prAction:             scm.ActionOpen,
			merged:               false,
			branchDeleteExpected: false,
		},
		{
			name:                 "Closed PR unmerged nothing to do",
			prAction:             scm.ActionClose,
			merged:               false,
			branchDeleteExpected: false,
		},
		{
			name:                 "PR from different repo nothing to do",
			prAction:             scm.ActionClose,
			merged:               true,
			headRepoFullName:     "different-org/repo",
			branchDeleteExpected: false,
		},
		{
			name:                 "PR from same repo delete head ref",
			prAction:             scm.ActionClose,
			merged:               true,
			headRepoFullName:     "my-org/repo",
			branchDeleteExpected: true,
		},
	}

	mergeSHA := "abc"
	prNumber := 1

	for _, tc := range testcases {

		t.Run(tc.name, func(t *testing.T) {
			log := logrus.WithField("plugin", pluginName)
			event := scm.PullRequestHook{
				Action: tc.prAction,
				PullRequest: scm.PullRequest{
					Number: prNumber,
					Base: scm.PullRequestBranch{
						Ref: "master",
						Repo: scm.Repository{
							Branch:    "master",
							FullName:  baseRepoFullName,
							Name:      baseRepoRepo,
							Namespace: baseRepoOrg,
						},
					},
					Head: scm.PullRequestBranch{
						Ref: "my-feature",
						Repo: scm.Repository{
							FullName: tc.headRepoFullName,
						},
					},
					Merged: tc.merged},
			}
			if tc.merged {
				event.PullRequest.MergeSha = mergeSHA
			}

			fakeScmClient, fgc := fake.NewDefault()
			fakeClient := scmprovider.ToTestClient(fakeScmClient)

			fgc.PullRequests[prNumber] = &scm.PullRequest{
				Number: prNumber,
			}
			if err := handle(fakeClient, log, event); err != nil {
				t.Fatalf("error in handle: %v", err)
			}
			if tc.branchDeleteExpected != (len(fgc.RefsDeleted) == 1) {
				t.Fatalf("branchDeleteExpected: %v, refsDeleted: %d", tc.branchDeleteExpected, len(fgc.RefsDeleted))
			}

			if tc.branchDeleteExpected {
				if fgc.RefsDeleted[0].Org != event.PullRequest.Base.Repo.Namespace {
					t.Errorf("Expected org of deleted ref to be %s but was %s", event.PullRequest.Base.Repo.Namespace, fgc.RefsDeleted[0].Org)
				}
				if fgc.RefsDeleted[0].Repo != event.PullRequest.Base.Repo.Name {
					t.Errorf("Expected repo of deleted ref to be %s but was %s", baseRepoRepo, fgc.RefsDeleted[0].Repo)
				}
				expectedRefName := fmt.Sprintf("heads/%s", event.PullRequest.Head.Ref)
				if fgc.RefsDeleted[0].Ref != expectedRefName {
					t.Errorf("Expected name of deleted ref to be %s but was %s", expectedRefName, fgc.RefsDeleted[0].Ref)
				}
			}

		})

	}
}
