/*
Copyright 2019 The Kubernetes Authors.

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

package bugzilla

import (
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/diff"
)

var (
	bugData   = []byte(`{"bugs":[{"alias":[],"assigned_to":"Steve Kuznetsov","assigned_to_detail":{"email":"skuznets","id":381851,"name":"skuznets","real_name":"Steve Kuznetsov"},"blocks":[],"cc":["Sudha Ponnaganti"],"cc_detail":[{"email":"sponnaga","id":426940,"name":"sponnaga","real_name":"Sudha Ponnaganti"}],"classification":"Red Hat","component":["Test Infrastructure"],"creation_time":"2019-05-01T19:33:36Z","creator":"Dan Mace","creator_detail":{"email":"dmace","id":330250,"name":"dmace","real_name":"Dan Mace"},"deadline":null,"depends_on":[],"docs_contact":"","dupe_of":null,"groups":[],"id":1705243,"is_cc_accessible":true,"is_confirmed":true,"is_creator_accessible":true,"is_open":true,"keywords":[],"last_change_time":"2019-05-17T15:13:13Z","op_sys":"Unspecified","platform":"Unspecified","priority":"unspecified","product":"OpenShift Container Platform","qa_contact":"","resolution":"","see_also":[],"severity":"medium","status":"VERIFIED","summary":"[ci] cli image flake affecting *-images jobs","target_milestone":"---","target_release":["3.11.z"],"url":"","version":["3.11.0"],"whiteboard":""}],"faults":[]}`)
	bugStruct = &Bug{Alias: []string{}, AssignedTo: "Steve Kuznetsov", AssignedToDetail: &User{Email: "skuznets", ID: 381851, Name: "skuznets", RealName: "Steve Kuznetsov"}, Blocks: []int{}, CC: []string{"Sudha Ponnaganti"}, CCDetail: []User{{Email: "sponnaga", ID: 426940, Name: "sponnaga", RealName: "Sudha Ponnaganti"}}, Classification: "Red Hat", Component: []string{"Test Infrastructure"}, CreationTime: "2019-05-01T19:33:36Z", Creator: "Dan Mace", CreatorDetail: &User{Email: "dmace", ID: 330250, Name: "dmace", RealName: "Dan Mace"}, DependsOn: []int{}, ID: 1705243, IsCCAccessible: true, IsConfirmed: true, IsCreatorAccessible: true, IsOpen: true, Groups: []string{}, Keywords: []string{}, LastChangeTime: "2019-05-17T15:13:13Z", OperatingSystem: "Unspecified", Platform: "Unspecified", Priority: "unspecified", Product: "OpenShift Container Platform", SeeAlso: []string{}, Severity: "medium", Status: "VERIFIED", Summary: "[ci] cli image flake affecting *-images jobs", TargetRelease: []string{"3.11.z"}, TargetMilestone: "---", Version: []string{"3.11.0"}}
)

func clientForUrl(url string) Client {
	return &client{
		logger:   logrus.WithField("testing", "true"),
		endpoint: url,
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
		getAPIKey: func() []byte {
			return []byte("api-key")
		},
	}
}

func TestGetBug(t *testing.T) {
	testServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-BUGZILLA-API-KEY") != "api-key" {
			t.Error("did not get api-key passed in X-BUGZILLA-API-KEY header")
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
		if r.URL.Query().Get("api_key") != "api-key" {
			t.Error("did not get api-key passed in api_key query parameter")
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("incorrect method to get a bug: %s", r.Method)
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/rest/bug/") {
			t.Errorf("incorrect path to get a bug: %s", r.URL.Path)
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
			return
		}
		if id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/rest/bug/")); err != nil {
			t.Errorf("malformed bug id: %s", r.URL.Path)
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
			return
		} else {
			if id == 1705243 {
				w.Write(bugData)
			} else {
				http.Error(w, "404 Not Found", http.StatusNotFound)
			}
		}
	}))
	defer testServer.Close()
	client := clientForUrl(testServer.URL)

	// this should give us what we want
	bug, err := client.GetBug(1705243)
	if err != nil {
		t.Errorf("expected no error, but got one: %v", err)
	}
	if !reflect.DeepEqual(bug, bugStruct) {
		t.Errorf("got incorrect bug: %v", diff.ObjectReflectDiff(bug, bugStruct))
	}

	// this should 404
	otherBug, err := client.GetBug(1)
	if err == nil {
		t.Error("expected an error, but got none")
	} else if !IsNotFound(err) {
		t.Errorf("expected a not found error, got %v", err)
	}
	if otherBug != nil {
		t.Errorf("expected no bug, got: %v", otherBug)
	}
}

func TestUpdateBug(t *testing.T) {
	testServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-BUGZILLA-API-KEY") != "api-key" {
			t.Error("did not get api-key passed in X-BUGZILLA-API-KEY header")
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("did not correctly set content-type header for JSON")
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
		if r.URL.Query().Get("api_key") != "api-key" {
			t.Error("did not get api-key passed in api_key query parameter")
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPut {
			t.Errorf("incorrect method to update a bug: %s", r.Method)
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/rest/bug/") {
			t.Errorf("incorrect path to update a bug: %s", r.URL.Path)
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
			return
		}
		if id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/rest/bug/")); err != nil {
			t.Errorf("malformed bug id: %s", r.URL.Path)
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
			return
		} else {
			if id == 1705243 {
				raw, err := ioutil.ReadAll(r.Body)
				if err != nil {
					t.Errorf("failed to read update body: %v", err)
				}
				if actual, expected := string(raw), `{"status":"UPDATED"}`; actual != expected {
					t.Errorf("got incorrect udpate: expected %v, got %v", expected, actual)
				}
			} else {
				http.Error(w, "404 Not Found", http.StatusNotFound)
			}
		}
	}))
	defer testServer.Close()
	client := clientForUrl(testServer.URL)

	// this should run an update
	if err := client.UpdateBug(1705243, BugUpdate{Status: "UPDATED"}); err != nil {
		t.Errorf("expected no error, but got one: %v", err)
	}

	// this should 404
	err := client.UpdateBug(1, BugUpdate{Status: "UPDATE"})
	if err == nil {
		t.Error("expected an error, but got none")
	} else if !IsNotFound(err) {
		t.Errorf("expected a not found error, got %v", err)
	}
}

func TestAddPullRequestAsExternalBug(t *testing.T) {
	var testCases = []struct {
		name            string
		id              int
		expectedPayload string
		response        string
		expectedError   bool
		expectedChanged bool
	}{
		{
			name:            "update succeeds, makes a change",
			id:              1705243,
			expectedPayload: `{"jsonrpc":"1.0","method":"ExternalBugs.add_external_bug","params":[{"api_key":"api-key","bug_ids":[1705243],"external_bugs":[{"ext_type_url":"https://github.com/","ext_bz_bug_id":"org/repo/pull/1"}]}],"id":"identifier"}`,
			response:        `{"error":null,"id":"identifier","result":{"bugs":[{"alias":[],"changes":{"ext_bz_bug_map.ext_bz_bug_id":{"added":"Github org/repo/pull/1","removed":""}},"id":1705243}]}}`,
			expectedError:   false,
			expectedChanged: true,
		},
		{
			name:            "update succeeds, makes a change as part of multiple changes reported",
			id:              1705244,
			expectedPayload: `{"jsonrpc":"1.0","method":"ExternalBugs.add_external_bug","params":[{"api_key":"api-key","bug_ids":[1705244],"external_bugs":[{"ext_type_url":"https://github.com/","ext_bz_bug_id":"org/repo/pull/1"}]}],"id":"identifier"}`,
			response:        `{"error":null,"id":"identifier","result":{"bugs":[{"alias":[],"changes":{"ext_bz_bug_map.ext_bz_bug_id":{"added":"Github org/repo/pull/1","removed":""}},"id":1705244},{"alias":[],"changes":{"ext_bz_bug_map.ext_bz_bug_id":{"added":"Github org/repo/pull/2","removed":""}},"id":1705244}]}}`,
			expectedError:   false,
			expectedChanged: true,
		},
		{
			name:            "update succeeds, makes no change",
			id:              1705245,
			expectedPayload: `{"jsonrpc":"1.0","method":"ExternalBugs.add_external_bug","params":[{"api_key":"api-key","bug_ids":[1705245],"external_bugs":[{"ext_type_url":"https://github.com/","ext_bz_bug_id":"org/repo/pull/1"}]}],"id":"identifier"}`,
			response:        `{"error":null,"id":"identifier","result":{"bugs":[]}}`,
			expectedError:   false,
			expectedChanged: false,
		},
		{
			name:            "update fails, makes no change",
			id:              1705246,
			expectedPayload: `{"jsonrpc":"1.0","method":"ExternalBugs.add_external_bug","params":[{"api_key":"api-key","bug_ids":[1705246],"external_bugs":[{"ext_type_url":"https://github.com/","ext_bz_bug_id":"org/repo/pull/1"}]}],"id":"identifier"}`,
			response:        `{"error":{"code": 100400,"message":"Invalid params for JSONRPC 1.0."},"id":"identifier","result":null}`,
			expectedError:   true,
			expectedChanged: false,
		},
		{
			name:            "get unrelated JSONRPC response",
			id:              1705247,
			expectedPayload: `{"jsonrpc":"1.0","method":"ExternalBugs.add_external_bug","params":[{"api_key":"api-key","bug_ids":[1705247],"external_bugs":[{"ext_type_url":"https://github.com/","ext_bz_bug_id":"org/repo/pull/1"}]}],"id":"identifier"}`,
			response:        `{"error":null,"id":"oops","result":{"bugs":[]}}`,
			expectedError:   true,
			expectedChanged: false,
		},
		{
			name:            "update already made earlier, makes no change",
			id:              1705248,
			expectedPayload: `{"jsonrpc":"1.0","method":"ExternalBugs.add_external_bug","params":[{"api_key":"api-key","bug_ids":[1705248],"external_bugs":[{"ext_type_url":"https://github.com/","ext_bz_bug_id":"org/repo/pull/1"}]}],"id":"identifier"}`,
			response:        `{"error":{"code": 100500,"message":"DBD::Pg::db do failed: ERROR:  duplicate key value violates unique constraint \"ext_bz_bug_map_bug_id_idx\"\nDETAIL:  Key (bug_id, ext_bz_id, ext_bz_bug_id)=(1778894, 131, openshift/installer/pull/2728) already exists. [for Statement \"INSERT INTO ext_bz_bug_map (ext_description, ext_bz_id, ext_bz_bug_id, ext_priority, ext_last_updated, bug_id, ext_status) VALUES (?,?,?,?,?,?,?)\"]\n\u003cpre\u003e\n at /var/www/html/bugzilla/Bugzilla/Object.pm line 754.\n\tBugzilla::Object::insert_create_data('Bugzilla::Extension::ExternalBugs::Bug', 'HASH(0x55eec2747a30)') called at /loader/0x55eec2720cc0/Bugzilla/Extension/ExternalBugs/Bug.pm line 118\n\tBugzilla::Extension::ExternalBugs::Bug::create('Bugzilla::Extension::ExternalBugs::Bug', 'HASH(0x55eed47b6d20)') called at /var/www/html/bugzilla/extensions/ExternalBugs/Extension.pm line 858\n\tBugzilla::Extension::ExternalBugs::bug_start_of_update('Bugzilla::Extension::ExternalBugs=HASH(0x55eecf484038)', 'HASH(0x55eed09302e8)') called at /var/www/html/bugzilla/Bugzilla/Hook.pm line 21\n\tBugzilla::Hook::process('bug_start_of_update', 'HASH(0x55eed09302e8)') called at /var/www/html/bugzilla/Bugzilla/Bug.pm line 1168\n\tBugzilla::Bug::update('Bugzilla::Bug=HASH(0x55eed048b350)') called at /loader/0x55eec2720cc0/Bugzilla/Extension/ExternalBugs/WebService.pm line 80\n\tBugzilla::Extension::ExternalBugs::WebService::add_external_bug('Bugzilla::WebService::Server::JSONRPC::Bugzilla::Extension::E...', 'HASH(0x55eed38bd710)') called at (eval 5435) line 1\n\teval ' $procedure-\u003e{code}-\u003e($self, @params) \n;' called at /usr/share/perl5/vendor_perl/JSON/RPC/Legacy/Server.pm line 220\n\tJSON::RPC::Legacy::Server::_handle('Bugzilla::WebService::Server::JSONRPC::Bugzilla::Extension::E...', 'HASH(0x55eed1990ef0)') called at /var/www/html/bugzilla/Bugzilla/WebService/Server/JSONRPC.pm line 295\n\tBugzilla::WebService::Server::JSONRPC::_handle('Bugzilla::WebService::Server::JSONRPC::Bugzilla::Extension::E...', 'HASH(0x55eed1990ef0)') called at /usr/share/perl5/vendor_perl/JSON/RPC/Legacy/Server.pm line 126\n\tJSON::RPC::Legacy::Server::handle('Bugzilla::WebService::Server::JSONRPC::Bugzilla::Extension::E...') called at /var/www/html/bugzilla/Bugzilla/WebService/Server/JSONRPC.pm line 70\n\tBugzilla::WebService::Server::JSONRPC::handle('Bugzilla::WebService::Server::JSONRPC::Bugzilla::Extension::E...') called at /var/www/html/bugzilla/jsonrpc.cgi line 31\n\tModPerl::ROOT::Bugzilla::ModPerl::ResponseHandler::var_www_html_bugzilla_jsonrpc_2ecgi::handler('Apache2::RequestRec=SCALAR(0x55eed3231870)') called at /usr/lib64/perl5/vendor_perl/ModPerl/RegistryCooker.pm line 207\n\teval {...} called at /usr/lib64/perl5/vendor_perl/ModPerl/RegistryCooker.pm line 207\n\tModPerl::RegistryCooker::run('Bugzilla::ModPerl::ResponseHandler=HASH(0x55eed023da08)') called at /usr/lib64/perl5/vendor_perl/ModPerl/RegistryCooker.pm line 173\n\tModPerl::RegistryCooker::default_handler('Bugzilla::ModPerl::ResponseHandler=HASH(0x55eed023da08)') called at /usr/lib64/perl5/vendor_perl/ModPerl/Registry.pm line 32\n\tModPerl::Registry::handler('Bugzilla::ModPerl::ResponseHandler', 'Apache2::RequestRec=SCALAR(0x55eed3231870)') called at /var/www/html/bugzilla/mod_perl.pl line 139\n\tBugzilla::ModPerl::ResponseHandler::handler('Bugzilla::ModPerl::ResponseHandler', 'Apache2::RequestRec=SCALAR(0x55eed3231870)') called at (eval 5435) line 0\n\teval {...} called at (eval 5435) line 0\n\n\u003c/pre\u003e at /var/www/html/bugzilla/Bugzilla/Object.pm line 754.\n at /var/www/html/bugzilla/Bugzilla/Object.pm line 754.\n\tBugzilla::Object::insert_create_data('Bugzilla::Extension::ExternalBugs::Bug', 'HASH(0x55eec2747a30)') called at /loader/0x55eec2720cc0/Bugzilla/Extension/ExternalBugs/Bug.pm line 118\n\tBugzilla::Extension::ExternalBugs::Bug::create('Bugzilla::Extension::ExternalBugs::Bug', 'HASH(0x55eed47b6d20)') called at /var/www/html/bugzilla/extensions/ExternalBugs/Extension.pm line 858\n\tBugzilla::Extension::ExternalBugs::bug_start_of_update('Bugzilla::Extension::ExternalBugs=HASH(0x55eecf484038)', 'HASH(0x55eed09302e8)') called at /var/www/html/bugzilla/Bugzilla/Hook.pm line 21\n\tBugzilla::Hook::process('bug_start_of_update', 'HASH(0x55eed09302e8)') called at /var/www/html/bugzilla/Bugzilla/Bug.pm line 1168\n\tBugzilla::Bug::update('Bugzilla::Bug=HASH(0x55eed048b350)') called at /loader/0x55eec2720cc0/Bugzilla/Extension/ExternalBugs/WebService.pm line 80\n\tBugzilla::Extension::ExternalBugs::WebService::add_external_bug('Bugzilla::WebService::Server::JSONRPC::Bugzilla::Extension::E...', 'HASH(0x55eed38bd710)') called at (eval 5435) line 1\n\teval ' $procedure-\u003e{code}-\u003e($self, @params) \n;' called at /usr/share/perl5/vendor_perl/JSON/RPC/Legacy/Server.pm line 220\n\tJSON::RPC::Legacy::Server::_handle('Bugzilla::WebService::Server::JSONRPC::Bugzilla::Extension::E...', 'HASH(0x55eed1990ef0)') called at /var/www/html/bugzilla/Bugzilla/WebService/Server/JSONRPC.pm line 295\n\tBugzilla::WebService::Server::JSONRPC::_handle('Bugzilla::WebService::Server::JSONRPC::Bugzilla::Extension::E...', 'HASH(0x55eed1990ef0)') called at /usr/share/perl5/vendor_perl/JSON/RPC/Legacy/Server.pm line 126\n\tJSON::RPC::Legacy::Server::handle('Bugzilla::WebService::Server::JSONRPC::Bugzilla::Extension::E...') called at /var/www/html/bugzilla/Bugzilla/WebService/Server/JSONRPC.pm line 70\n\tBugzilla::WebService::Server::JSONRPC::handle('Bugzilla::WebService::Server::JSONRPC::Bugzilla::Extension::E...') called at /var/www/html/bugzilla/jsonrpc.cgi line 31\n\tModPerl::ROOT::Bugzilla::ModPerl::ResponseHandler::var_www_html_bugzilla_jsonrpc_2ecgi::handler('Apache2::RequestRec=SCALAR(0x55eed3231870)') called at /usr/lib64/perl5/vendor_perl/ModPerl/RegistryCooker.pm line 207\n\teval {...} called at /usr/lib64/perl5/vendor_perl/ModPerl/RegistryCooker.pm line 207\n\tModPerl::RegistryCooker::run('Bugzilla::ModPerl::ResponseHandler=HASH(0x55eed023da08)') called at /usr/lib64/perl5/vendor_perl/ModPerl/RegistryCooker.pm line 173\n\tModPerl::RegistryCooker::default_handler('Bugzilla::ModPerl::ResponseHandler=HASH(0x55eed023da08)') called at /usr/lib64/perl5/vendor_perl/ModPerl/Registry.pm line 32\n\tModPerl::Registry::handler('Bugzilla::ModPerl::ResponseHandler', 'Apache2::RequestRec=SCALAR(0x55eed3231870)') called at /var/www/html/bugzilla/mod_perl.pl line 139\n\tBugzilla::ModPerl::ResponseHandler::handler('Bugzilla::ModPerl::ResponseHandler', 'Apache2::RequestRec=SCALAR(0x55eed3231870)') called at (eval 5435) line 0\n\teval {...} called at (eval 5435) line 0"},"id":"identifier","result":null}`,
			expectedError:   false,
			expectedChanged: false,
		},
	}
	testServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Error("did not correctly set content-type header for JSON")
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("incorrect method to use the JSONRPC API: %s", r.Method)
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
			return
		}
		if r.URL.Path != "/jsonrpc.cgi" {
			t.Errorf("incorrect path to use the JSONRPC API: %s", r.URL.Path)
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
			return
		}
		var payload struct {
			// Version is the version of JSONRPC to use. All Bugzilla servers
			// support 1.0. Some support 1.1 and some support 2.0
			Version string `json:"jsonrpc"`
			Method  string `json:"method"`
			// Parameters must be specified in JSONRPC 1.0 as a structure in the first
			// index of this slice
			Parameters []AddExternalBugParameters `json:"params"`
			ID         string                     `json:"id"`
		}
		raw, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Errorf("failed to read request body: %v", err)
			http.Error(w, "500 Server Error", http.StatusInternalServerError)
			return
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Errorf("malformed JSONRPC payload: %s", string(raw))
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
			return
		}
		for _, testCase := range testCases {
			if payload.Parameters[0].BugIDs[0] == testCase.id {
				if actual, expected := string(raw), testCase.expectedPayload; actual != expected {
					t.Errorf("%s: got incorrect JSONRPC payload: %v", testCase.name, diff.ObjectReflectDiff(expected, actual))
				}
				if _, err := w.Write([]byte(testCase.response)); err != nil {
					t.Fatalf("%s: failed to send JSONRPC response: %v", testCase.name, err)
				}
				return
			}
		}
		http.Error(w, "404 Not Found", http.StatusNotFound)
	}))
	defer testServer.Close()
	client := clientForUrl(testServer.URL)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			changed, err := client.AddPullRequestAsExternalBug(testCase.id, "org", "repo", 1)
			if !testCase.expectedError && err != nil {
				t.Errorf("%s: expected no error, but got one: %v", testCase.name, err)
			}
			if testCase.expectedError && err == nil {
				t.Errorf("%s: expected an error, but got none", testCase.name)
			}
			if testCase.expectedChanged != changed {
				t.Errorf("%s: got incorrect state change", testCase.name)
			}
		})
	}

	// this should 404
	changed, err := client.AddPullRequestAsExternalBug(1, "org", "repo", 1)
	if err == nil {
		t.Error("expected an error, but got none")
	} else if !IsNotFound(err) {
		t.Errorf("expected a not found error, got %v", err)
	}
	if changed {
		t.Error("expected not to change state, but did")
	}
}

func TestIdentifierForPull(t *testing.T) {
	var testCases = []struct {
		name      string
		org, repo string
		num       int
		expected  string
	}{
		{
			name:     "normal works as expected",
			org:      "organization",
			repo:     "repository",
			num:      1234,
			expected: "organization/repository/pull/1234",
		},
	}

	for _, testCase := range testCases {
		if actual, expected := IdentifierForPull(testCase.org, testCase.repo, testCase.num), testCase.expected; actual != expected {
			t.Errorf("%s: got incorrect identifier, expected %s but got %s", testCase.name, expected, actual)
		}
	}
}

func TestPullFromIdentifier(t *testing.T) {
	var testCases = []struct {
		name                      string
		identifier                string
		expectedOrg, expectedRepo string
		expectedNum               int
		expectedErr               bool
		expectedNotPullErr        bool
	}{
		{
			name:         "normal works as expected",
			identifier:   "organization/repository/pull/1234",
			expectedOrg:  "organization",
			expectedRepo: "repository",
			expectedNum:  1234,
		},
		{
			name:        "wrong number of parts fails",
			identifier:  "organization/repository",
			expectedErr: true,
		},
		{
			name:               "not a pull fails but in an identifiable way",
			identifier:         "organization/repository/issue/1234",
			expectedErr:        true,
			expectedNotPullErr: true,
		},
		{
			name:        "not a number fails",
			identifier:  "organization/repository/pull/abcd",
			expectedErr: true,
		},
	}

	for _, testCase := range testCases {
		org, repo, num, err := PullFromIdentifier(testCase.identifier)
		if testCase.expectedErr && err == nil {
			t.Errorf("%s: expected an error but got none", testCase.name)
		}
		if !testCase.expectedErr && err != nil {
			t.Errorf("%s: expected no error but got one: %v", testCase.name, err)
		}
		if testCase.expectedNotPullErr && !IsIdentifierNotForPullErr(err) {
			t.Errorf("%s: expected a notForPull error but got: %T", testCase.name, err)
		}
		if org != testCase.expectedOrg {
			t.Errorf("%s: got incorrect org, expected %s but got %s", testCase.name, testCase.expectedOrg, org)
		}
		if repo != testCase.expectedRepo {
			t.Errorf("%s: got incorrect repo, expected %s but got %s", testCase.name, testCase.expectedRepo, repo)
		}
		if num != testCase.expectedNum {
			t.Errorf("%s: got incorrect num, expected %d but got %d", testCase.name, testCase.expectedNum, num)
		}
	}
}

func TestGetExternalBugPRsOnBug(t *testing.T) {
	var testCases = []struct {
		name          string
		id            int
		response      string
		expectedError bool
		expectedPRs   []ExternalBug
	}{
		{
			name:          "no external bugs returns empty list",
			id:            1705243,
			response:      `{"bugs":[{"external_bugs":[]}],"faults":[]}`,
			expectedError: false,
		},
		{
			name:          "one external bug pointing to PR is found",
			id:            1705244,
			response:      `{"bugs":[{"external_bugs":[{"bug_id": 1705244,"ext_bz_bug_id":"org/repo/pull/1","type":{"url":"https://github.com/"}}]}],"faults":[]}`,
			expectedError: false,
			expectedPRs:   []ExternalBug{{Type: ExternalBugType{URL: "https://github.com/"}, BugzillaBugID: 1705244, ExternalBugID: "org/repo/pull/1", Org: "org", Repo: "repo", Num: 1}},
		},
		{
			name:          "multiple external bugs pointing to PRs are found",
			id:            1705245,
			response:      `{"bugs":[{"external_bugs":[{"bug_id": 1705245,"ext_bz_bug_id":"org/repo/pull/1","type":{"url":"https://github.com/"}},{"bug_id": 1705245,"ext_bz_bug_id":"org/repo/pull/2","type":{"url":"https://github.com/"}}]}],"faults":[]}`,
			expectedError: false,
			expectedPRs:   []ExternalBug{{Type: ExternalBugType{URL: "https://github.com/"}, BugzillaBugID: 1705245, ExternalBugID: "org/repo/pull/1", Org: "org", Repo: "repo", Num: 1}, {Type: ExternalBugType{URL: "https://github.com/"}, BugzillaBugID: 1705245, ExternalBugID: "org/repo/pull/2", Org: "org", Repo: "repo", Num: 2}},
		},
		{
			name:          "external bugs pointing to issues are ignored",
			id:            1705246,
			response:      `{"bugs":[{"external_bugs":[{"bug_id": 1705246,"ext_bz_bug_id":"org/repo/issues/1","type":{"url":"https://github.com/"}}]}],"faults":[]}`,
			expectedError: false,
		},
		{
			name:          "external bugs pointing to other Bugzilla bugs are ignored",
			id:            1705247,
			response:      `{"bugs":[{"external_bugs":[{"bug_id": 3,"ext_bz_bug_id":"org/repo/pull/1","type":{"url":"https://github.com/"}}]}],"faults":[]}`,
			expectedError: false,
		},
		{
			name:          "external bugs pointing to other trackers are ignored",
			id:            1705248,
			response:      `{"bugs":[{"external_bugs":[{"bug_id": 1705248,"ext_bz_bug_id":"something","type":{"url":"https://bugs.tracker.com/"}}]}],"faults":[]}`,
			expectedError: false,
		},
		{
			name:          "external bugs pointing to invalid pulls cause an error",
			id:            1705249,
			response:      `{"bugs":[{"external_bugs":[{"bug_id": 1705249,"ext_bz_bug_id":"org/repo/pull/c","type":{"url":"https://github.com/"}}]}],"faults":[]}`,
			expectedError: true,
		},
	}
	testServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-BUGZILLA-API-KEY") != "api-key" {
			t.Error("did not get api-key passed in X-BUGZILLA-API-KEY header")
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
		if r.URL.Query().Get("api_key") != "api-key" {
			t.Error("did not get api-key passed in api_key query parameter")
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
		if r.URL.Query().Get("include_fields") != "external_bugs" {
			t.Error("did not get external bugs passed in include_fields query parameter")
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("incorrect method to get a bug: %s", r.Method)
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/rest/bug/") {
			t.Errorf("incorrect path to get a bug: %s", r.URL.Path)
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
			return
		}
		if id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/rest/bug/")); err != nil {
			t.Errorf("malformed bug id: %s", r.URL.Path)
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
			return
		} else {
			for _, testCase := range testCases {
				if id == testCase.id {
					if _, err := w.Write([]byte(testCase.response)); err != nil {
						t.Fatalf("%s: failed to send response: %v", testCase.name, err)
					}
					return
				}
			}
		}
	}))
	defer testServer.Close()
	client := clientForUrl(testServer.URL)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			prs, err := client.GetExternalBugPRsOnBug(testCase.id)
			if !testCase.expectedError && err != nil {
				t.Errorf("%s: expected no error, but got one: %v", testCase.name, err)
			}
			if testCase.expectedError && err == nil {
				t.Errorf("%s: expected an error, but got none", testCase.name)
			}
			if actual, expected := prs, testCase.expectedPRs; !reflect.DeepEqual(actual, expected) {
				t.Errorf("%s: got incorrect prs: %v", testCase.name, diff.ObjectReflectDiff(actual, expected))
			}
		})
	}
}

func TestGetExternalBugs(t *testing.T) {
	var testCases = []struct {
		name          string
		id            int
		response      string
		expectedError bool
		expectedBugs  []ExternalBug
	}{
		{
			name:          "no external bugs returns empty list",
			id:            1705243,
			response:      `{"bugs":[{"external_bugs":[]}],"faults":[]}`,
			expectedError: false,
		},
		{
			name:          "one external bug pointing to PR is found",
			id:            1705244,
			response:      `{"bugs":[{"external_bugs":[{"bug_id": 1705244,"ext_bz_bug_id":"org/repo/pull/1","type":{"url":"https://github.com/"}}]}],"faults":[]}`,
			expectedError: false,
			expectedBugs:  []ExternalBug{{Type: ExternalBugType{URL: "https://github.com/"}, BugzillaBugID: 1705244, ExternalBugID: "org/repo/pull/1"}},
		},
		{
			name:          "multiple external bugs pointing to PRs are found",
			id:            1705245,
			response:      `{"bugs":[{"external_bugs":[{"bug_id": 1705245,"ext_bz_bug_id":"org/repo/pull/1","type":{"url":"https://github.com/"}},{"bug_id": 1705245,"ext_bz_bug_id":"org/repo/pull/2","type":{"url":"https://github.com/"}}]}],"faults":[]}`,
			expectedError: false,
			expectedBugs:  []ExternalBug{{Type: ExternalBugType{URL: "https://github.com/"}, BugzillaBugID: 1705245, ExternalBugID: "org/repo/pull/1"}, {Type: ExternalBugType{URL: "https://github.com/"}, BugzillaBugID: 1705245, ExternalBugID: "org/repo/pull/2"}},
		},
		{
			name:          "external bugs pointing to issues show up, but don't contain repo, org, num metadata",
			id:            1705246,
			response:      `{"bugs":[{"external_bugs":[{"bug_id": 1705246,"ext_bz_bug_id":"org/repo/issues/1","type":{"url":"https://github.com/"}}]}],"faults":[]}`,
			expectedError: false,
			expectedBugs:  []ExternalBug{{Type: ExternalBugType{URL: "https://github.com/"}, BugzillaBugID: 1705246, ExternalBugID: "org/repo/issues/1"}},
		},
		{
			name:          "external bugs pointing to other Bugzilla bugs are ignored",
			id:            1705247,
			response:      `{"bugs":[{"external_bugs":[{"bug_id": 3,"ext_bz_bug_id":"org/repo/pull/1","type":{"url":"https://github.com/"}}]}],"faults":[]}`,
			expectedError: false,
		},
		{
			name:          "external bugs pointing to other trackers are show up, but don't contain repo, org, num metadata",
			id:            1705248,
			response:      `{"bugs":[{"external_bugs":[{"bug_id": 1705248,"ext_bz_bug_id":"something","type":{"url":"https://bugs.tracker.com/"}}]}],"faults":[]}`,
			expectedError: false,
			expectedBugs:  []ExternalBug{{Type: ExternalBugType{URL: "https://bugs.tracker.com/"}, BugzillaBugID: 1705248, ExternalBugID: "something"}},
		},
		{
			name:          "external bugs pointing to invalid pulls are ignored",
			id:            1705249,
			response:      `{"bugs":[{"external_bugs":[{"bug_id": 1705249,"ext_bz_bug_id":"org/repo/pull/c","type":{"url":"https://github.com/"}}]}],"faults":[]}`,
			expectedError: false,
			expectedBugs:  []ExternalBug{{Type: ExternalBugType{URL: "https://github.com/"}, BugzillaBugID: 1705249, ExternalBugID: "org/repo/pull/c"}},
		},
	}
	testServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-BUGZILLA-API-KEY") != "api-key" {
			t.Error("did not get api-key passed in X-BUGZILLA-API-KEY header")
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
		if r.URL.Query().Get("api_key") != "api-key" {
			t.Error("did not get api-key passed in api_key query parameter")
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
		if r.URL.Query().Get("include_fields") != "external_bugs" {
			t.Error("did not get external bugs passed in include_fields query parameter")
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
			return
		}
		if r.Method != http.MethodGet {
			t.Errorf("incorrect method to get a bug: %s", r.Method)
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
			return
		}
		if !strings.HasPrefix(r.URL.Path, "/rest/bug/") {
			t.Errorf("incorrect path to get a bug: %s", r.URL.Path)
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
			return
		}
		if id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/rest/bug/")); err != nil {
			t.Errorf("malformed bug id: %s", r.URL.Path)
			http.Error(w, "400 Bad Request", http.StatusBadRequest)
			return
		} else {
			for _, testCase := range testCases {
				if id == testCase.id {
					if _, err := w.Write([]byte(testCase.response)); err != nil {
						t.Fatalf("%s: failed to send response: %v", testCase.name, err)
					}
					return
				}
			}
		}
	}))
	defer testServer.Close()
	client := clientForUrl(testServer.URL)

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			prs, err := client.GetExternalBugs(testCase.id)
			if !testCase.expectedError && err != nil {
				t.Errorf("%s: expected no error, but got one: %v", testCase.name, err)
			}
			if testCase.expectedError && err == nil {
				t.Errorf("%s: expected an error, but got none", testCase.name)
			}
			if actual, expected := prs, testCase.expectedBugs; !reflect.DeepEqual(actual, expected) {
				t.Errorf("%s: got incorrect prs: %v", testCase.name, diff.ObjectReflectDiff(actual, expected))
			}
		})
	}
}

type authExpected struct {
	bearer bool
	query  bool
	xbug   bool
	err    bool
}

func testAuth(t *testing.T, authMethod string, expected authExpected) {
	testServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-BUGZILLA-API-KEY")
		if key != "api-key" && expected.xbug {
			t.Error("did not get api-key passed in X-BUGZILLA-API-KEY header")
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
		if key != "" && !expected.xbug {
			t.Error("Incorrectly sent X-BUGZILLA-API-KEY header")
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}

		key = r.Header.Get("Authorization")
		if key != "Bearer api-key" && expected.bearer {
			t.Errorf("did not get api-key passed in Authorization header")
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
		if key != "" && !expected.bearer {
			t.Error("Incorrectly sent Authorization Bearer header")
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}

		key = r.URL.Query().Get("api_key")
		if key != "api-key" && expected.query {
			t.Error("did not get api-key passed in api_key query parameter")
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
		if key != "" && !expected.query {
			t.Error("Incorrectly sent auth in Query")
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
	}))
	defer testServer.Close()

	client := clientForUrl(testServer.URL)
	err := client.SetAuthMethod(authMethod)
	if err != nil {
		if expected.err {
			return // SUCCESS!
		}
		t.Errorf("Got error setting auth method: %v", err)
	}

	_, _ = client.GetBug(1)
}

func TestAuth(t *testing.T) {
	var testCases = map[string]struct {
		method   string
		expected authExpected
	}{
		AuthBearer: {
			method: AuthBearer,
			expected: authExpected{
				bearer: true,
			},
		},
		AuthQuery: {
			method: AuthQuery,
			expected: authExpected{
				query: true,
			},
		},
		AuthXBugzillaAPIKey: {
			method: AuthXBugzillaAPIKey,
			expected: authExpected{
				xbug: true,
			},
		},
		"garbagein": {
			method: "garbagein",
			expected: authExpected{
				err: true,
			},
		},
		"no auth": {
			method: "",
			expected: authExpected{
				query: true,
				xbug:  true,
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			testAuth(t, tc.method, tc.expected)
		})
	}
}
