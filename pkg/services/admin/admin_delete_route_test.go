/*
Copyright 2023 Avi Zimmerman <avi.zimmerman@gmail.com>

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

package admin

import (
	"testing"

	v1 "github.com/webmeshproj/api/v1"
	"google.golang.org/grpc/codes"
)

func TestDeleteRoute(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	tc := []testCase[v1.Route]{
		{
			name: "no route name",
			code: codes.InvalidArgument,
			req:  &v1.Route{},
		},
		{
			name: "any route name",
			code: codes.OK,
			req:  &v1.Route{Name: "foo"},
		},
	}

	runTestCases(t, tc, server.DeleteRoute)
}
