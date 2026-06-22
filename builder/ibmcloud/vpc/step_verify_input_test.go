package vpc

import (
	"errors"
	"testing"

	"github.com/IBM/go-sdk-core/v5/core"
	searchv2 "github.com/IBM/platform-services-go-sdk/globalsearchv2"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

// fakeSearcher stands in for *globalsearchv2.GlobalSearchV2.
//
//   - err != nil      => every Search returns (nil, nil, err), modelling a
//     transport/auth failure (the real SDK leaves result nil on error).
//   - results != nil  => each call returns the next entry in order, modelling
//     several CRN lookups where a later one differs from earlier ones.
//   - otherwise       => every call returns result.
//
// Every query string seen is recorded in queries, so a test can assert the
// lookup query verifyCRN builds from the CRN.
type fakeSearcher struct {
	result  *searchv2.ScanResult
	results []*searchv2.ScanResult
	err     error
	calls   int
	queries []string
}

func (f *fakeSearcher) Search(opts *searchv2.SearchOptions) (*searchv2.ScanResult, *core.DetailedResponse, error) {
	if opts != nil && opts.Query != nil {
		f.queries = append(f.queries, *opts.Query)
	}
	if f.err != nil {
		return nil, nil, f.err
	}
	if f.results != nil {
		r := f.results[f.calls]
		f.calls++
		return r, nil, nil
	}
	return f.result, nil, nil
}

// resolved is a non-nil result carrying one item — a CRN that exists.
func resolved() *searchv2.ScanResult {
	item := searchv2.ResultItem{}
	item.SetProperty("name", "test-resource")
	return &searchv2.ScanResult{Items: []searchv2.ResultItem{item}}
}

// notFound is the real API's "not found" shape: a non-nil result with no items.
func notFound() *searchv2.ScanResult {
	return &searchv2.ScanResult{}
}

// assertErrorFails checks the build was made to fail: the step halted and the
// "error" state key holds a non-nil error (Packer core reads that key — a
// present-but-nil or non-error value would not fail the build).
func assertErrorFails(t *testing.T, action multistep.StepAction, state *multistep.BasicStateBag) {
	t.Helper()
	if action != multistep.ActionHalt {
		t.Fatalf("expected ActionHalt, got %v", action)
	}
	v, ok := state.GetOk("error")
	if !ok {
		t.Fatal(`verification failure must set the "error" state key so Packer core fails the build`)
	}
	if err, isErr := v.(error); !isErr || err == nil {
		t.Fatalf(`"error" state key must hold a non-nil error, got %T (%v)`, v, v)
	}
}

// An unresolved CRN must fail the build, not pass silently. The regression: the
// failure has to land under the "error" state key (Packer core reads that key to
// decide the build failed), and the step must halt.
func TestVerifyCRNs_UnresolvedCRNFailsBuild(t *testing.T) {
	cases := map[string]Config{
		"catalog offering": {CatalogOfferingCRN: "crn:v1:bluemix:public:globalcatalog::::offering:x"},
		"catalog version":  {CatalogOfferingVersionCRN: "crn:v1:bluemix:public:globalcatalog::::version:x"},
	}
	for name, config := range cases {
		t.Run(name, func(t *testing.T) {
			state := new(multistep.BasicStateBag)

			action := verifyCRNs(&fakeSearcher{result: notFound()}, config, packer.TestUi(t), state)

			assertErrorFails(t, action, state)
		})
	}
}

// A Global Search transport/auth error must fail the build cleanly — not panic
// (the SDK returns a nil result on error) and not be misreported as "not found".
func TestVerifyCRNs_SearchErrorFailsBuild(t *testing.T) {
	state := new(multistep.BasicStateBag)
	config := Config{CatalogOfferingCRN: "crn:v1:bluemix:public:globalcatalog::::offering:x"}

	action := verifyCRNs(&fakeSearcher{err: errors.New("403 Forbidden")}, config, packer.TestUi(t), state)

	assertErrorFails(t, action, state)
}

// Defensive: a nil result with no error must be treated as a failure, not
// dereferenced.
func TestVerifyCRNs_NilResultFailsBuild(t *testing.T) {
	state := new(multistep.BasicStateBag)
	config := Config{CatalogOfferingCRN: "crn:v1:bluemix:public:globalcatalog::::offering:x"}

	action := verifyCRNs(&fakeSearcher{result: nil}, config, packer.TestUi(t), state)

	assertErrorFails(t, action, state)
}

func TestVerifyCRNs_ResolvedCRNContinues(t *testing.T) {
	cases := map[string]Config{
		"catalog offering": {CatalogOfferingCRN: "crn:v1:bluemix:public:globalcatalog::::offering:x"},
		"catalog version":  {CatalogOfferingVersionCRN: "crn:v1:bluemix:public:globalcatalog::::version:x"},
	}
	for name, config := range cases {
		t.Run(name, func(t *testing.T) {
			state := new(multistep.BasicStateBag)

			action := verifyCRNs(&fakeSearcher{result: resolved()}, config, packer.TestUi(t), state)

			if action != multistep.ActionContinue {
				t.Fatalf("expected ActionContinue for a resolved CRN, got %v", action)
			}
			if _, ok := state.GetOk("error"); ok {
				t.Fatal("a resolved CRN must not set the error state key")
			}
		})
	}
}

// The lookup query is the CRN truncated at its type segment (":offering",
// ":version") with "::" appended — so each CRN kind must use its own separator
// and produce the expected prefix query.
func TestVerifyCRNs_BuildsPrefixQuery(t *testing.T) {
	cases := []struct {
		name      string
		config    Config
		wantQuery string
	}{
		{
			"catalog offering",
			Config{CatalogOfferingCRN: "crn:v1:bluemix:public:globalcatalog:global:a/acct:cat-id:offering:off-id"},
			`crn:"crn:v1:bluemix:public:globalcatalog:global:a/acct:cat-id::"`,
		},
		{
			"catalog version",
			Config{CatalogOfferingVersionCRN: "crn:v1:bluemix:public:globalcatalog:global:a/acct:cat-id:version:ver-id"},
			`crn:"crn:v1:bluemix:public:globalcatalog:global:a/acct:cat-id::"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			search := &fakeSearcher{result: resolved()}

			verifyCRNs(search, tc.config, packer.TestUi(t), new(multistep.BasicStateBag))

			if len(search.queries) != 1 {
				t.Fatalf("expected exactly 1 Search query, got %d: %v", len(search.queries), search.queries)
			}
			if search.queries[0] != tc.wantQuery {
				t.Errorf("query mismatch:\n got: %s\nwant: %s", search.queries[0], tc.wantQuery)
			}
		})
	}
}

// With both catalog CRNs set, an unresolved one checked last must still halt and
// set "error" — i.e. an earlier success doesn't short-circuit the later check.
func TestVerifyCRNs_LaterCRNFailureHalts(t *testing.T) {
	state := new(multistep.BasicStateBag)
	config := Config{
		CatalogOfferingCRN:        "crn:v1:bluemix:public:globalcatalog::::offering:x",
		CatalogOfferingVersionCRN: "crn:v1:bluemix:public:globalcatalog::::version:x",
	}
	search := &fakeSearcher{results: []*searchv2.ScanResult{resolved(), notFound()}}

	action := verifyCRNs(search, config, packer.TestUi(t), state)

	assertErrorFails(t, action, state)
	if search.calls != 2 {
		t.Fatalf("expected both CRNs checked in order, got %d Search calls", search.calls)
	}
}

func TestVerifyCRNs_NoCRNConfigured(t *testing.T) {
	state := new(multistep.BasicStateBag)

	action := verifyCRNs(&fakeSearcher{}, Config{}, packer.TestUi(t), state)

	if action != multistep.ActionContinue {
		t.Fatalf("expected ActionContinue when no CRN is configured, got %v", action)
	}
}
