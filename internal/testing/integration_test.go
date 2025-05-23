//go:build integration

package testing

import (
	"context"
	"flag"
	"net/netip"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/libdns/libdns"
	"github.com/libdns/namecheap"
)

var (
	apiKey      = flag.String("api-key", "", "Namecheap API key.")
	apiUser     = flag.String("username", "", "Namecheap API username.")
	apiEndpoint = flag.String("endpoint", "https://api.sandbox.namecheap.com/xml.response", "Namecheap API endpoint.")
	domain      = flag.String("domain", "", "Domain to test with of the form sld.tld <testing.com>")
	clientIP    = flag.String("client-ip", "", "Public IP address of client machine")
)

var thirtyMinutes = time.Minute * 30

// cleanupRecords deletes all records for the given domain.
func cleanupRecords(t *testing.T, p *namecheap.Provider, domain string) {
	t.Helper()
	t.Logf("Cleaning up records for %s", domain)
	records, err := p.GetRecords(context.TODO(), domain)
	if err != nil {
		t.Fatalf("Failed to get records for cleanup: %v", err)
	}
	if len(records) > 0 {
		t.Logf("Found %d records to clean up: %#v", len(records), records)
		for _, record := range records {
			t.Logf("Deleting record: %#v", record)
		}
		_, err = p.DeleteRecords(context.TODO(), domain, records)
		if err != nil {
			t.Fatalf("Failed to delete records during cleanup: %v", err)
		}
		// Verify cleanup
		remainingRecords, err := p.GetRecords(context.TODO(), domain)
		if err != nil {
			t.Fatalf("Failed to verify cleanup: %v", err)
		}
		if len(remainingRecords) > 0 {
			t.Fatalf("Cleanup failed: %d records still remain: %#v", len(remainingRecords), remainingRecords)
		}
		t.Logf("Successfully cleaned up all records")
	} else {
		t.Logf("No records found to clean up")
	}
}

func TestIntegration(t *testing.T) {
	p := &namecheap.Provider{
		APIKey:      *apiKey,
		User:        *apiUser,
		APIEndpoint: *apiEndpoint,
		ClientIP:    *clientIP,
	}

	cleanupRecords(t, p, *domain)
	t.Cleanup(func() {
		cleanupRecords(t, p, *domain)
	})

	newRecords := []libdns.Record{
		&libdns.Address{
			Name: "@",
			IP:   netip.MustParseAddr("127.0.0.1"),
			TTL:  thirtyMinutes,
		},
		&libdns.Address{
			Name: "www",
			IP:   netip.MustParseAddr("127.0.0.1"),
			TTL:  thirtyMinutes,
		},
	}

	t.Logf("Appending: %d Records", len(newRecords))

	addedRecords, err := p.AppendRecords(context.TODO(), *domain, newRecords)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Records appended: %#v", addedRecords)

	records, err := p.GetRecords(context.TODO(), *domain)
	if err != nil {
		t.Fatal(err)
	}

	if len(records) != len(newRecords) {
		t.Fatalf("Expected %d records, got %d", len(newRecords), len(records))
	}

	// Sort records by name to ensure consistent comparison
	sort.Slice(records, func(i, j int) bool {
		return records[i].(*libdns.Address).Name < records[j].(*libdns.Address).Name
	})
	sort.Slice(addedRecords, func(i, j int) bool {
		return addedRecords[i].(*libdns.Address).Name < addedRecords[j].(*libdns.Address).Name
	})

	if diff := cmp.Diff(addedRecords, records, cmpopts.EquateComparable(netip.Addr{})); diff != "" {
		t.Fatalf("Added records not equal to fetched records. Diff: %s", diff)
	}

	firstRecord := records[0]

	t.Logf("Removing record: %#v", firstRecord)
	recordsRemoved, err := p.DeleteRecords(context.TODO(), *domain, []libdns.Record{firstRecord})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Records removed: %#v", recordsRemoved)

	records, err = p.GetRecords(context.TODO(), *domain)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Final number of records: %d", len(records))

	if len(records) != 1 {
		t.Fatalf("Expected 1 but got: %v", len(records))
	}
}

func TestSetRecordsKeepsExisting(t *testing.T) {
	p := &namecheap.Provider{
		APIKey:      *apiKey,
		User:        *apiUser,
		APIEndpoint: *apiEndpoint,
		ClientIP:    *clientIP,
	}

	cleanupRecords(t, p, *domain)
	t.Cleanup(func() {
		cleanupRecords(t, p, *domain)
	})

	newRecords := []libdns.Record{
		&libdns.Address{
			Name: "@",
			IP:   netip.MustParseAddr("127.0.0.1"),
			TTL:  thirtyMinutes,
		},
		&libdns.Address{
			Name: "www",
			IP:   netip.MustParseAddr("127.0.0.1"),
			TTL:  thirtyMinutes,
		},
	}

	t.Logf("Appending: %d Records", len(newRecords))

	addedRecords, err := p.AppendRecords(context.TODO(), *domain, newRecords)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Records appended: %#v", addedRecords)

	records, err := p.GetRecords(context.TODO(), *domain)
	if err != nil {
		t.Fatal(err)
	}

	if len(records) != 2 {
		t.Fatalf("Expected 2 records. Got: %d", len(records))
	}

	if rr, ok := records[0].(*libdns.Address); ok {
		rr.IP = netip.MustParseAddr("0.0.0.0")
	}

	t.Log("Updating record")
	updatedRecords, err := p.SetRecords(context.TODO(), *domain, records)
	if err != nil {
		t.Fatal(err)
	}

	if len(updatedRecords) != 2 {
		t.Fatalf("Expected 2 records. Got: %d", len(updatedRecords))
	}

	updatedRecords, err = p.GetRecords(context.TODO(), *domain)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Updated records: %#v", updatedRecords)

	if len(updatedRecords) != 2 {
		t.Fatalf("Expected 2 records. Got: %d", len(updatedRecords))
	}

	var updatedRecord *libdns.Address
	for _, record := range updatedRecords {
		if rr, ok := record.(*libdns.Address); ok && rr.IP.String() == "0.0.0.0" {
			updatedRecord = rr
			break
		}
	}

	if updatedRecord == nil {
		t.Fatal("Record was never updated.")
	}
}

func TestAllRecordTypes(t *testing.T) {
	p := &namecheap.Provider{
		APIKey:      *apiKey,
		User:        *apiUser,
		APIEndpoint: *apiEndpoint,
		ClientIP:    *clientIP,
	}

	cleanupRecords(t, p, *domain)
	t.Cleanup(func() {
		cleanupRecords(t, p, *domain)
	})

	newRecords := []libdns.Record{
		&libdns.Address{
			Name: "a",
			TTL:  thirtyMinutes,
			IP:   netip.MustParseAddr("127.0.0.1"),
		},
		&libdns.CNAME{
			Name:   "www",
			TTL:    thirtyMinutes,
			Target: *domain,
		},
		&libdns.TXT{
			Name: "txt",
			TTL:  thirtyMinutes,
			Text: "test text",
		},
		&libdns.MX{
			Name:       "mx",
			TTL:        thirtyMinutes,
			Preference: 10,
			Target:     "mail.example.com",
		},
		&libdns.NS{
			Name:   "ns",
			TTL:    thirtyMinutes,
			Target: "ns1.example.com.",
		},
		&libdns.CAA{
			Name:  "@",
			TTL:   thirtyMinutes,
			Flags: 0,
			Tag:   "issue",
			Value: "letsencrypt.org",
		},
		&libdns.RR{
			Type: "ALIAS",
			Name: "alias",
			Data: "example.com.",
			TTL:  time.Minute,
		},
	}

	t.Logf("Appending: %d Records", len(newRecords))

	addedRecords, err := p.AppendRecords(context.TODO(), *domain, newRecords)
	if err != nil {
		t.Fatal(err)
	}

	records, err := p.GetRecords(context.TODO(), *domain)
	if err != nil {
		t.Fatal(err)
	}

	for _, record := range addedRecords {
		// Namecheap adds quotes around the value even if we don't provide them
		// so adding them here so that the returned records from Append/Delete match
		// those from GetRecords.
		if rr, ok := record.(*libdns.CAA); ok {
			rr.Value = strconv.Quote(rr.Value)
		}
		// Namecheap adds a '.' to the end of CNAME records.
		if rr, ok := record.(*libdns.CNAME); ok {
			rr.Target = rr.Target + "."
		}
	}

	sortRecordsFunc := func(a, b libdns.Record) int {
		ar := a.RR()
		br := b.RR()

		if ar.Name != br.Name {
			return strings.Compare(ar.Name, br.Name)
		} else if ar.Type != br.Type {
			return strings.Compare(ar.Type, br.Type)
		} else if ar.Data != br.Data {
			return strings.Compare(ar.Data, br.Data)
		} else if ar.TTL != br.TTL {
			return int(ar.TTL.Seconds() - br.TTL.Seconds())
		}
		return 0
	}

	slices.SortFunc(addedRecords, sortRecordsFunc)
	slices.SortFunc(records, sortRecordsFunc)

	if diff := cmp.Diff(addedRecords, records, cmpopts.EquateComparable(netip.Addr{})); diff != "" {
		t.Fatalf("Added records not equal to fetched records. Diff: %s", diff)
	}

	_, err = p.DeleteRecords(context.TODO(), *domain, records)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Records deleted: %#v", records)

	records, err = p.GetRecords(context.TODO(), *domain)
	if err != nil {
		t.Fatal(err)
	}

	if len(records) != 0 {
		t.Fatalf("Expected 0 records. Got: %d", len(records))
	}
}
