package namecheap

import (
	"context"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/libdns/libdns"
	"github.com/libdns/namecheap/internal/namecheap"
)

var (
	thirtyMinutes = time.Duration(30 * time.Minute)
	oneHour       = time.Duration(1 * time.Hour)
)

func TestSetRecordsUpdatesExisting(t *testing.T) {
	testCases := map[string]struct {
		existingRecords []libdns.Record
		recordsToUpdate []libdns.Record
		expectedRecords []libdns.Record
		zone            string
	}{
		"update existing record address": {
			existingRecords: []libdns.Record{
				&libdns.Address{
					Name: "@",
					TTL:  thirtyMinutes,
					IP:   mustParseIP("1.2.3.4"),
				},
				&libdns.Address{
					Name: "www",
					TTL:  thirtyMinutes,
					IP:   mustParseIP("122.23.3.7"),
				},
			},
			recordsToUpdate: []libdns.Record{
				&libdns.Address{
					Name: "@",
					TTL:  thirtyMinutes,
					IP:   mustParseIP("0.0.0.0"),
				},
			},
			expectedRecords: []libdns.Record{
				&libdns.Address{
					Name: "www",
					TTL:  thirtyMinutes,
					IP:   mustParseIP("122.23.3.7"),
				},
				&libdns.Address{
					Name: "@",
					TTL:  thirtyMinutes,
					IP:   mustParseIP("0.0.0.0"),
				},
			},
			zone: "domain.com.",
		},
		"replace a records keep txt": {
			existingRecords: []libdns.Record{
				&libdns.Address{
					Name: "@",
					TTL:  oneHour,
					IP:   mustParseIP("192.0.2.1"),
				},
				&libdns.Address{
					Name: "@",
					TTL:  oneHour,
					IP:   mustParseIP("192.0.2.2"),
				},
				&libdns.TXT{
					Name: "@",
					TTL:  oneHour,
					Text: "hello world",
				},
			},
			recordsToUpdate: []libdns.Record{
				&libdns.Address{
					Name: "@",
					TTL:  oneHour,
					IP:   mustParseIP("192.0.2.3"),
				},
			},
			expectedRecords: []libdns.Record{
				&libdns.TXT{
					Name: "@",
					TTL:  oneHour,
					Text: "hello world",
				},
				&libdns.Address{
					Name: "@",
					TTL:  oneHour,
					IP:   mustParseIP("192.0.2.3"),
				},
			},
			zone: "example.com.",
		},
		"update alpha keep beta": {
			existingRecords: []libdns.Record{
				&libdns.Address{
					Name: "alpha",
					TTL:  oneHour,
					IP:   mustParseIP("2001:db8::1"),
				},
				&libdns.Address{
					Name: "alpha",
					TTL:  oneHour,
					IP:   mustParseIP("2001:db8::2"),
				},
				&libdns.Address{
					Name: "beta",
					TTL:  oneHour,
					IP:   mustParseIP("2001:db8::3"),
				},
				&libdns.Address{
					Name: "beta",
					TTL:  oneHour,
					IP:   mustParseIP("2001:db8::4"),
				},
			},
			recordsToUpdate: []libdns.Record{
				&libdns.Address{
					Name: "alpha",
					TTL:  oneHour,
					IP:   mustParseIP("2001:db8::1"),
				},
				&libdns.Address{
					Name: "alpha",
					TTL:  oneHour,
					IP:   mustParseIP("2001:db8::2"),
				},
				&libdns.Address{
					Name: "alpha",
					TTL:  oneHour,
					IP:   mustParseIP("2001:db8::5"),
				},
			},
			expectedRecords: []libdns.Record{
				&libdns.Address{
					Name: "beta",
					TTL:  oneHour,
					IP:   mustParseIP("2001:db8::3"),
				},
				&libdns.Address{
					Name: "beta",
					TTL:  oneHour,
					IP:   mustParseIP("2001:db8::4"),
				},
				&libdns.Address{
					Name: "alpha",
					TTL:  oneHour,
					IP:   mustParseIP("2001:db8::1"),
				},
				&libdns.Address{
					Name: "alpha",
					TTL:  oneHour,
					IP:   mustParseIP("2001:db8::2"),
				},
				&libdns.Address{
					Name: "alpha",
					TTL:  oneHour,
					IP:   mustParseIP("2001:db8::5"),
				},
			},
			zone: "example.com.",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ts := namecheap.SetupTestServer(t, convertToHostRecords(tc.existingRecords)...)

			provider := &Provider{
				APIKey:      "testAPIKey",
				User:        "testUser",
				APIEndpoint: ts.URL,
				ClientIP:    "localhost",
			}

			_, err := provider.SetRecords(context.TODO(), tc.zone, tc.recordsToUpdate)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}

			records, err := provider.GetRecords(context.TODO(), tc.zone)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}

			if diff := cmp.Diff(tc.expectedRecords, records, cmpopts.EquateComparable(netip.Addr{})); diff != "" {
				t.Fatalf("Expected records does not match: %s", diff)
			}
		})
	}
}

func TestDeleteRecordsWithExisting(t *testing.T) {
	testCases := map[string]struct {
		existingRecords []libdns.Record
		recordsToDelete []libdns.Record
		expectedRecords []libdns.Record
		zone            string
	}{
		"delete www record": {
			existingRecords: []libdns.Record{
				&libdns.Address{
					Name: "@",
					TTL:  thirtyMinutes,
					IP:   mustParseIP("1.2.3.4"),
				},
				&libdns.Address{
					Name: "www",
					TTL:  thirtyMinutes,
					IP:   mustParseIP("122.23.3.7"),
				},
			},
			recordsToDelete: []libdns.Record{
				&libdns.Address{
					Name: "www",
					TTL:  thirtyMinutes,
					IP:   mustParseIP("122.23.3.7"),
				},
			},
			expectedRecords: []libdns.Record{
				&libdns.Address{
					Name: "@",
					TTL:  thirtyMinutes,
					IP:   mustParseIP("1.2.3.4"),
				},
			},
			zone: "domain.com.",
		},
		"non-existing record does not delete existing record": {
			existingRecords: []libdns.Record{
				&libdns.Address{
					Name: "@",
					TTL:  thirtyMinutes,
					IP:   mustParseIP("1.2.3.4"),
				},
			},
			recordsToDelete: []libdns.Record{
				&libdns.Address{
					Name: "www",
					TTL:  thirtyMinutes,
					IP:   mustParseIP("122.23.3.7"),
				},
			},
			expectedRecords: []libdns.Record{
				&libdns.Address{
					Name: "@",
					TTL:  thirtyMinutes,
					IP:   mustParseIP("1.2.3.4"),
				},
			},
			zone: "domain.co.uk.",
		},
		"partial match does not delete record": {
			existingRecords: []libdns.Record{
				&libdns.Address{
					Name: "@",
					TTL:  thirtyMinutes,
					IP:   mustParseIP("1.2.3.4"),
				},
			},
			recordsToDelete: []libdns.Record{
				&libdns.Address{
					Name: "@",
					TTL:  thirtyMinutes,
					IP:   mustParseIP("1.1.1.1"),
				},
			},
			expectedRecords: []libdns.Record{
				&libdns.Address{
					Name: "@",
					TTL:  thirtyMinutes,
					IP:   mustParseIP("1.2.3.4"),
				},
			},
			zone: "domain.co.uk.",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ts := namecheap.SetupTestServer(t, convertToHostRecords(tc.existingRecords)...)

			provider := &Provider{
				APIKey:      "testAPIKey",
				User:        "testUser",
				APIEndpoint: ts.URL,
				ClientIP:    "localhost",
			}

			_, err := provider.DeleteRecords(context.TODO(), tc.zone, tc.recordsToDelete)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}

			records, err := provider.GetRecords(context.TODO(), tc.zone)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}

			if diff := cmp.Diff(tc.expectedRecords, records, cmpopts.EquateComparable(netip.Addr{})); diff != "" {
				t.Fatalf("Expected records does not match: %s", diff)
			}
		})
	}
}

func TestConcurrentDeleteRecords(t *testing.T) {
	ts := namecheap.SetupTestServer(t)

	// Create provider with mock endpoint
	provider := &Provider{
		APIKey:      "test-key",
		User:        "test-user",
		APIEndpoint: ts.URL,
		ClientIP:    "127.0.0.1",
	}

	zone := "example.com."

	record1 := &libdns.Address{
		Name: "test1",
		TTL:  30 * time.Minute,
		IP:   mustParseIP("1.1.1.1"),
	}
	record2 := &libdns.Address{
		Name: "test2",
		TTL:  30 * time.Minute,
		IP:   mustParseIP("2.2.2.2"),
	}
	record3 := &libdns.Address{
		Name: "test3",
		TTL:  30 * time.Minute,
		IP:   mustParseIP("3.3.3.3"),
	}

	_, err := provider.AppendRecords(context.Background(), zone, []libdns.Record{record1, record2, record3})
	if err != nil {
		t.Fatalf("Failed to add initial records: %v", err)
	}

	// Get the records because the IDs are not set in the AppendRecords response.
	records, err := provider.GetRecords(context.Background(), zone)
	if err != nil {
		t.Fatalf("Failed to get records: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		if _, err := provider.DeleteRecords(context.Background(), zone, []libdns.Record{records[0]}); err != nil {
			t.Errorf("First DeleteRecords failed: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		if _, err := provider.DeleteRecords(context.Background(), zone, []libdns.Record{records[1]}); err != nil {
			t.Errorf("Second DeleteRecords failed: %v", err)
		}
	}()

	wg.Wait()

	remainingRecords, err := provider.GetRecords(context.Background(), zone)
	if err != nil {
		t.Fatalf("Failed to get remaining records: %v", err)
	}

	if len(remainingRecords) != 1 {
		t.Errorf("Expected 1 remaining record, got %d", len(remainingRecords))
	}
}

func TestConcurrentSetRecords(t *testing.T) {
	ts := namecheap.SetupTestServer(t)

	provider := &Provider{
		APIKey:      "test-key",
		User:        "test-user",
		APIEndpoint: ts.URL,
		ClientIP:    "127.0.0.1",
	}

	zone := "example.com."

	record1 := &libdns.Address{
		Name: "test1",
		TTL:  30 * time.Minute,
		IP:   mustParseIP("1.1.1.1"),
	}

	_, err := provider.SetRecords(context.Background(), zone, []libdns.Record{record1})
	if err != nil {
		t.Fatalf("Failed to add initial record: %v", err)
	}

	records, err := provider.GetRecords(context.Background(), zone)
	if err != nil {
		t.Fatalf("Failed to get records: %v", err)
	}

	modifiedRecord1 := &libdns.Address{
		Name: records[0].RR().Name,
		TTL:  records[0].RR().TTL,
		IP:   mustParseIP("1.1.1.2"),
	}

	modifiedRecord1Again := &libdns.Address{
		Name: records[0].RR().Name,
		TTL:  records[0].RR().TTL,
		IP:   mustParseIP("1.1.1.3"),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, err := provider.SetRecords(context.Background(), zone, []libdns.Record{modifiedRecord1})
		if err != nil {
			t.Errorf("First SetRecords failed: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		_, err := provider.SetRecords(context.Background(), zone, []libdns.Record{modifiedRecord1Again})
		if err != nil {
			t.Errorf("Second SetRecords failed: %v", err)
		}
	}()

	wg.Wait()

	remainingRecords, err := provider.GetRecords(context.Background(), zone)
	if err != nil {
		t.Fatalf("Failed to get remaining records: %v", err)
	}

	if len(remainingRecords) != 1 {
		t.Errorf("Expected 1 record, got %d", len(remainingRecords))
	}
}

func TestAppendRecords(t *testing.T) {
	threeMinutes := 3 * time.Minute
	testCases := map[string]struct {
		existingRecords []libdns.Record
		recordsToAppend []libdns.Record
		expectedRecords []libdns.Record
		zone            string
	}{
		"append two address records": {
			recordsToAppend: []libdns.Record{
				&libdns.Address{
					Name: "first_host",
					TTL:  threeMinutes,
					IP:   netip.IPv4Unspecified(),
				},
				&libdns.Address{
					Name: "second_host",
					TTL:  0,
					IP:   netip.IPv4Unspecified(),
				},
			},
			expectedRecords: []libdns.Record{
				&libdns.Address{
					Name: "first_host",
					TTL:  threeMinutes,
					IP:   netip.IPv4Unspecified(),
				},
				&libdns.Address{
					Name: "second_host",
					TTL:  0,
					IP:   netip.IPv4Unspecified(),
				},
			},
			zone: "domain.com.",
		},
		"append single record with default TTL": {
			existingRecords: []libdns.Record{
				&libdns.Address{
					Name: "existing_host",
					TTL:  30 * time.Minute,
					IP:   mustParseIP("192.168.1.1"),
				},
			},
			recordsToAppend: []libdns.Record{
				&libdns.Address{
					Name: "new_host",
					TTL:  0,
					IP:   netip.IPv4Unspecified(),
				},
			},
			expectedRecords: []libdns.Record{
				&libdns.Address{
					Name: "existing_host",
					TTL:  30 * time.Minute,
					IP:   mustParseIP("192.168.1.1"),
				},
				&libdns.Address{
					Name: "new_host",
					TTL:  0,
					IP:   netip.IPv4Unspecified(),
				},
			},
			zone: "example.com.",
		},
		"append mixed record types": {
			recordsToAppend: []libdns.Record{
				&libdns.Address{
					Name: "mixed_host",
					TTL:  5 * time.Minute,
					IP:   netip.IPv4Unspecified(),
				},
				&libdns.TXT{
					Name: "mixed_txt",
					TTL:  10 * time.Minute,
					Text: "test text",
				},
			},
			expectedRecords: []libdns.Record{
				&libdns.Address{
					Name: "mixed_host",
					TTL:  5 * time.Minute,
					IP:   netip.IPv4Unspecified(),
				},
				&libdns.TXT{
					Name: "mixed_txt",
					TTL:  10 * time.Minute,
					Text: "test text",
				},
			},
			zone: "mixed.com.",
		},
		"append existing record with different TTL should not return record": {
			existingRecords: []libdns.Record{
				&libdns.Address{
					Name: "existing",
					TTL:  thirtyMinutes,
					IP:   mustParseIP("192.168.1.1"),
				},
			},
			recordsToAppend: []libdns.Record{
				&libdns.Address{
					Name: "existing",
					TTL:  oneHour,
					IP:   mustParseIP("192.168.1.1"),
				},
			},
			expectedRecords: []libdns.Record{
				&libdns.Address{
					Name: "existing",
					TTL:  thirtyMinutes,
					IP:   mustParseIP("192.168.1.1"),
				},
			},
			zone: "example.com.",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			ts := namecheap.SetupTestServer(t, convertToHostRecords(tc.existingRecords)...)

			provider := &Provider{
				APIKey:      "testAPIKey",
				User:        "testUser",
				APIEndpoint: ts.URL,
				ClientIP:    "localhost",
			}

			records, err := provider.AppendRecords(context.Background(), tc.zone, tc.recordsToAppend)
			if err != nil {
				t.Fatalf("Unexpected error: %s", err)
			}
			if diff := cmp.Diff(tc.recordsToAppend, records, cmpopts.EquateComparable(netip.Addr{})); diff != "" {
				t.Fatalf("Expected records does not match: %s", diff)
			}

			allRecords, err := provider.GetRecords(context.Background(), tc.zone)
			if err != nil {
				t.Fatalf("Failed to get records: %s", err)
			}

			if diff := cmp.Diff(tc.expectedRecords, allRecords, cmpopts.EquateComparable(netip.Addr{})); diff != "" {
				t.Fatalf("Expected records does not match: %s", diff)
			}
		})
	}
}

func TestAppendRecordsConcurrent(t *testing.T) {
	ts := namecheap.SetupTestServer(t)

	provider := &Provider{
		APIKey:      "test-key",
		User:        "test-user",
		APIEndpoint: ts.URL,
		ClientIP:    "127.0.0.1",
	}

	zone := "example.com"

	record1 := &libdns.Address{
		Name: "test1",
		TTL:  thirtyMinutes,
		IP:   mustParseIP("1.1.1.1"),
	}
	record2 := &libdns.Address{
		Name: "test2",
		TTL:  thirtyMinutes,
		IP:   mustParseIP("2.2.2.2"),
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		_, err := provider.AppendRecords(context.Background(), zone, []libdns.Record{record1})
		if err != nil {
			t.Errorf("First AppendRecords failed: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		_, err := provider.AppendRecords(context.Background(), zone, []libdns.Record{record2})
		if err != nil {
			t.Errorf("Second AppendRecords failed: %v", err)
		}
	}()

	wg.Wait()

	remainingRecords, err := provider.GetRecords(context.Background(), zone)
	if err != nil {
		t.Fatalf("Failed to get remaining records: %v", err)
	}

	if len(remainingRecords) != 2 {
		t.Errorf("Expected 2 records, got %d", len(remainingRecords))
	}

	recordMap := make(map[string]bool)
	for _, r := range remainingRecords {
		rr := r.RR()
		if addr, ok := r.(*libdns.Address); ok {
			recordMap[rr.Name+addr.IP.String()] = true
		}
	}

	if !recordMap[record1.RR().Name+record1.IP.String()] {
		t.Errorf("record1 (%s) was not added", record1.RR().Name+record1.IP.String())
	}
	if !recordMap[record2.RR().Name+record2.IP.String()] {
		t.Errorf("record2 (%s) was not added", record2.RR().Name+record2.IP.String())
	}
}

func convertToHostRecords(records []libdns.Record) []namecheap.HostRecord {
	var hostRecords []namecheap.HostRecord
	for _, r := range records {
		hostRecords = append(hostRecords, parseIntoHostRecord(r))
	}
	return hostRecords
}

// Helper function to parse IP addresses for test data
func mustParseIP(ipStr string) netip.Addr {
	ip, err := netip.ParseAddr(ipStr)
	if err != nil {
		panic(err)
	}
	return ip
}
