package namecheap_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/libdns/namecheap/internal/namecheap"
)

const (
	setHostsResponse = `<?xml version="1.0" encoding="UTF-8"?>
<ApiResponse xmlns="https://api.namecheap.com/xml.response" Status="OK">
  <Errors />
  <RequestedCommand>namecheap.domains.dns.setHosts</RequestedCommand>
  <CommandResponse Type="namecheap.domains.dns.setHosts">
    <DomainDNSSetHostsResult Domain="domain51.com" IsSuccess="true" />
  </CommandResponse>
  <Server>SERVER-NAME</Server>
  <GMTTimeDifference>+5</GMTTimeDifference>
  <ExecutionTime>32.76</ExecutionTime>
</ApiResponse>`

	getHostsResponse = `<?xml version="1.0" encoding="UTF-8"?>
<ApiResponse xmlns="http://api.namecheap.com/xml.response" Status="OK">
  <Errors />
  <RequestedCommand>namecheap.domains.dns.getHosts</RequestedCommand>
  <CommandResponse Type="namecheap.domains.dns.getHosts">
    <DomainDNSGetHostsResult Domain="domain.com" IsUsingOurDNS="true">
      <Host HostId="12" Name="@" Type="A" Address="1.2.3.4" MXPref="10" TTL="1800" />
      <Host HostId="14" Name="www" Type="A" Address="122.23.3.7" MXPref="10" TTL="1800" />
    </DomainDNSGetHostsResult>
  </CommandResponse>
  <Server>SERVER-NAME</Server>
  <GMTTimeDifference>+5</GMTTimeDifference>
  <ExecutionTime>32.76</ExecutionTime>
</ApiResponse>`

	emptyHostsResponse = `<?xml version="1.0" encoding="UTF-8"?>
<ApiResponse xmlns="http://api.namecheap.com/xml.response" Status="OK">
  <Errors />
  <RequestedCommand>namecheap.domains.dns.getHosts</RequestedCommand>
  <CommandResponse Type="namecheap.domains.dns.getHosts">
    <DomainDNSGetHostsResult Domain="domain.com" IsUsingOurDNS="true" />
  </CommandResponse>
  <Server>SERVER-NAME</Server>
  <GMTTimeDifference>+5</GMTTimeDifference>
  <ExecutionTime>32.76</ExecutionTime>
</ApiResponse>`

	errorResponse = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
  <Errors>
    <Error Number="1010102">Parameter APIKey is missing</Error>
  </Errors>
  <Warnings />
  <RequestedCommand />
  <Server>TEST111</Server>
  <GMTTimeDifference>--1:00</GMTTimeDifference>
  <ExecutionTime>0</ExecutionTime>
</ApiResponse>`

	getTLDListResponse = `<?xml version="1.0" encoding="UTF-8"?>
<ApiResponse xmlns="http://api.namecheap.com/xml.response" Status="OK">
  <Errors />
  <RequestedCommand>namecheap.domains.getTldList</RequestedCommand>
  <CommandResponse Type="namecheap.domains.getTldList">
    <Tlds>
      <Tld Name="biz" NonRealTime="false" MinRegisterYears="1" MaxRegisterYears="10" MinRenewYears="1" MaxRenewYears="10" MinTransferYears="1" MaxTransferYears="10" IsApiRegisterable="true" IsApiRenewable="true" IsApiTransferable="false" IsEppRequired="false" IsDisableModContact="false" IsDisableWGAllot="false" IsIncludeInExtendedSearchOnly="false" SequenceNumber="5" Type="GTLD" IsSupportsIDN="false" Category="P">US Business</Tld>
      <Tld Name="bz" NonRealTime="false" MinRegisterYears="1" MaxRegisterYears="10" MinRenewYears="1" MaxRenewYears="10" MinTransferYears="1" MaxTransferYears="10" IsApiRegisterable="false" IsApiRenewable="false" IsApiTransferable="false" IsEppRequired="false" IsDisableModContact="false" IsDisableWGAllot="false" IsIncludeInExtendedSearchOnly="true" SequenceNumber="11" Type="CCTLD" IsSupportsIDN="false" Category="A">BZ Country Domain</Tld>
      <Tld Name="ca" NonRealTime="true" MinRegisterYears="1" MaxRegisterYears="10" MinRenewYears="1" MaxRenewYears="10" MinTransferYears="1" MaxTransferYears="10" IsApiRegisterable="false" IsApiRenewable="false" IsApiTransferable="false" IsEppRequired="false" IsDisableModContact="false" IsDisableWGAllot="false" IsIncludeInExtendedSearchOnly="true" SequenceNumber="7" Type="CCTLD" IsSupportsIDN="false" Category="A">Canada Country TLD</Tld>
      <Tld Name="cc" NonRealTime="false" MinRegisterYears="1" MaxRegisterYears="10" MinRenewYears="1" MaxRenewYears="10" MinTransferYears="1" MaxTransferYears="10" IsApiRegisterable="false" IsApiRenewable="false" IsApiTransferable="false" IsEppRequired="false" IsDisableModContact="false" IsDisableWGAllot="false" IsIncludeInExtendedSearchOnly="true" SequenceNumber="9" Type="CCTLD" IsSupportsIDN="false" Category="A">CC TLD</Tld>
      <Tld Name="co.uk" NonRealTime="false" MinRegisterYears="2" MaxRegisterYears="10" MinRenewYears="2" MaxRenewYears="10" MinTransferYears="2" MaxTransferYears="10" IsApiRegisterable="true" IsApiRenewable="false" IsApiTransferable="false" IsEppRequired="false" IsDisableModContact="false" IsDisableWGAllot="false" IsIncludeInExtendedSearchOnly="false" SequenceNumber="18" Type="CCTLD" IsSupportsIDN="false" Category="A">UK based domain</Tld>
      <Tld Name="com" NonRealTime="false" MinRegisterYears="1" MaxRegisterYears="10" MinRenewYears="1" MaxRenewYears="10" MinTransferYears="1" MaxTransferYears="10" IsApiRegisterable="true" IsApiRenewable="true" IsApiTransferable="true" IsEppRequired="false" IsDisableModContact="false" IsDisableWGAllot="false" IsIncludeInExtendedSearchOnly="false" SequenceNumber="1" Type="GTLD" IsSupportsIDN="false" Category="G">COM Generic Top-level Domain</Tld>
    </Tlds>
  </CommandResponse>
  <Server>IMWS-A06</Server>
  <GMTTimeDifference>+5:30</GMTTimeDifference>
  <ExecutionTime>0.047</ExecutionTime>
</ApiResponse>`
)

func ensureBody(t *testing.T, r *http.Request, expectedBody string) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(expectedBody, string(body)); diff != "" {
		t.Fatalf("Expected body does not match received: %s", diff)
	}
}

func toURLValues(values map[string]string) url.Values {
	urlValues := make(url.Values)
	for k, v := range values {
		urlValues[k] = []string{v}
	}
	return urlValues
}

func TestGetHosts(t *testing.T) {
	expectedValues := map[string]string{
		"ApiUser":  "testUser",
		"ApiKey":   "testAPIKey",
		"UserName": "testUser",
		"ClientIp": "localhost",
		"Command":  "namecheap.domains.dns.getHosts",
		"TLD":      "domain",
		"SLD":      "any",
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ensureBody(t, r, toURLValues(expectedValues).Encode())
		_, err := w.Write([]byte(getHostsResponse))
		if err != nil {
			t.Fatal(err)
		}
	}))
	t.Cleanup(ts.Close)

	c, err := namecheap.NewClient("testAPIKey", "testUser", namecheap.WithEndpoint(ts.URL), namecheap.WithClientIP("localhost"))
	if err != nil {
		t.Fatalf("Error creating NewClient. Err: %s", err)
	}

	hosts, err := c.GetHosts(context.TODO(), namecheap.Domain{
		TLD: "domain",
		SLD: "any",
	})
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	expectedHosts := map[string]namecheap.HostRecord{
		"12": {
			Name:       "@",
			HostID:     "12",
			RecordType: namecheap.A,
			Address:    "1.2.3.4",
			MXPref:     "10",
			TTL:        1800,
		},
		"14": {
			Name:       "www",
			HostID:     "14",
			RecordType: namecheap.A,
			Address:    "122.23.3.7",
			MXPref:     "10",
			TTL:        1800,
		},
	}

	if len(hosts) != len(expectedHosts) {
		t.Fatalf("Length does not match expected. Expected: %d. Got: %d.", len(expectedHosts), len(hosts))
	}

	for _, host := range hosts {
		if host.HostID == "" {
			t.Fatal("Empty HostID")
		}

		if diff := cmp.Diff(host, expectedHosts[host.HostID]); diff != "" {
			t.Fatalf("Host and expected host are not equal. Diff: %s", diff)
		}
	}
}

func TestGetHostsContextCanceled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			w.Write([]byte(errorResponse))
		case <-time.After(time.Second):
			t.Fatal("Context was not cancelled in time")
		}
	}))
	t.Cleanup(ts.Close)

	c, err := namecheap.NewClient("testAPIKey", "testUser", namecheap.WithEndpoint(ts.URL), namecheap.WithClientIP("localhost"))
	if err != nil {
		t.Fatalf("Error creating NewClient. Err: %s", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.GetHosts(ctx, namecheap.Domain{
		TLD: "domain",
		SLD: "any",
	}); err == nil {
		t.Fatal("Expected error cancelling context but got none")
	}
}

func TestGetHostsWithExtraDotInDomain(t *testing.T) {
	expectedValues := map[string]string{
		"ApiUser":  "testUser",
		"ApiKey":   "testAPIKey",
		"UserName": "testUser",
		"ClientIp": "localhost",
		"Command":  "namecheap.domains.dns.getHosts",
		"TLD":      "domain",
		"SLD":      "any",
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ensureBody(t, r, toURLValues(expectedValues).Encode())
		_, err := w.Write([]byte(getHostsResponse))
		if err != nil {
			t.Fatal(err)
		}
	}))
	t.Cleanup(ts.Close)

	c, err := namecheap.NewClient("testAPIKey", "testUser", namecheap.WithEndpoint(ts.URL), namecheap.WithClientIP("localhost"))
	if err != nil {
		t.Fatalf("Error creating NewClient. Err: %s", err)
	}

	if _, err := c.GetHosts(context.TODO(), namecheap.Domain{
		TLD: "domain",
		SLD: "any",
	}); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
}

func TestGetHostsWithExtraDotsInTLD(t *testing.T) {
	expectedValues := map[string]string{
		"ApiUser":  "testUser",
		"ApiKey":   "testAPIKey",
		"UserName": "testUser",
		"ClientIp": "localhost",
		"Command":  "namecheap.domains.dns.getHosts",
		"TLD":      "co.uk",
		"SLD":      "any",
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ensureBody(t, r, toURLValues(expectedValues).Encode())
		_, err := w.Write([]byte(getHostsResponse))
		if err != nil {
			t.Fatal(err)
		}
	}))
	t.Cleanup(ts.Close)

	c, err := namecheap.NewClient("testAPIKey", "testUser", namecheap.WithEndpoint(ts.URL), namecheap.WithClientIP("localhost"))
	if err != nil {
		t.Fatalf("Error creating NewClient. Err: %s", err)
	}

	if _, err := c.GetHosts(context.TODO(), namecheap.Domain{
		TLD: "co.uk",
		SLD: "any",
	}); err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
}

func TestSetHosts(t *testing.T) {
	expected := map[string]string{
		"ApiUser":     "testUser",
		"ApiKey":      "testAPIKey",
		"UserName":    "testUser",
		"ClientIp":    "localhost",
		"Command":     "namecheap.domains.dns.setHosts",
		"TLD":         "com",
		"SLD":         "domain",
		"HostName1":   "first_host",
		"RecordType1": string(namecheap.A),
		"TTL1":        "180",
		"HostName2":   "second_host",
		"RecordType2": string(namecheap.A),
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			ensureBody(t, r, toURLValues(expected).Encode())
			w.Write([]byte(setHostsResponse))
		case http.MethodGet:
			w.Write([]byte(emptyHostsResponse))
		}
	}))
	t.Cleanup(ts.Close)
	c, err := namecheap.NewClient("testAPIKey", "testUser", namecheap.WithEndpoint(ts.URL), namecheap.WithClientIP("localhost"))
	if err != nil {
		t.Fatalf("Error creating NewClient. Err: %s", err)
	}

	hosts := []namecheap.HostRecord{
		{
			Name:       "first_host",
			RecordType: namecheap.A,
			TTL:        uint16(180),
		},
		{
			Name:       "second_host",
			RecordType: namecheap.A,
		},
	}

	_, err = c.SetHosts(context.TODO(), namecheap.Domain{
		TLD: "com",
		SLD: "domain",
	}, hosts)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
}

func TestGetHostsError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(errorResponse))
	}))
	t.Cleanup(ts.Close)

	c, err := namecheap.NewClient("testAPIKey", "testUser", namecheap.WithEndpoint(ts.URL), namecheap.WithClientIP("localhost"))
	if err != nil {
		t.Fatalf("Error creating NewClient. Err: %s", err)
	}

	_, err = c.GetHosts(context.TODO(), namecheap.Domain{
		TLD: "domain",
		SLD: "any",
	})
	if err == nil {
		t.Fatal("Expected error but got nil")
	}
}

func TestBadURL(t *testing.T) {
	c, err := namecheap.NewClient("testAPIKey", "testUser", namecheap.WithEndpoint("any"), namecheap.WithClientIP("localhost"))
	if err != nil {
		t.Fatalf("Error creating NewClient. Err: %s", err)
	}

	_, err = c.GetHosts(context.TODO(), namecheap.Domain{
		TLD: "com",
		SLD: "any",
	})
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	_, err = c.GetHosts(context.TODO(), namecheap.Domain{})
	if err == nil {
		t.Fatal("Expected error but got nil")
	}
}

func TestAutoDiscoverIP(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.String(), "getHosts") {
			if got := r.URL.Query().Get("ClientIp"); got != "127.0.0.1" {
				t.Fatalf("Expected: %s\tGot: %s", "127.0.0.1", got)
			}
		}
		w.Write([]byte("127.0.0.1"))
	}))
	t.Cleanup(ts.Close)

	c, err := namecheap.NewClient("testAPIKey", "testUser", namecheap.AutoDiscoverPublicIP(), namecheap.WithDiscoveryAddress(ts.URL), namecheap.WithEndpoint(ts.URL))
	if err != nil {
		t.Fatalf("Error creating NewClient. Err: %s", err)
	}

	c.GetHosts(context.TODO(), namecheap.Domain{
		TLD: "domain",
		SLD: "any",
	})
}

func TestGetTLDs(t *testing.T) {
	expectedValues := map[string]string{
		"ApiUser":  "testUser",
		"ApiKey":   "testAPIKey",
		"UserName": "testUser",
		"ClientIp": "localhost",
		"Command":  "namecheap.domains.getTldList",
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ensureBody(t, r, toURLValues(expectedValues).Encode())
		_, err := w.Write([]byte(getTLDListResponse))
		if err != nil {
			t.Fatal(err)
		}
	}))
	t.Cleanup(ts.Close)

	c, err := namecheap.NewClient("testAPIKey", "testUser", namecheap.WithEndpoint(ts.URL), namecheap.WithClientIP("localhost"))
	if err != nil {
		t.Fatalf("Error creating NewClient. Err: %s", err)
	}

	tlds, err := c.GetTLDs(context.TODO())
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}

	expectedTLDs := []namecheap.TLD{
		{Name: "biz"},
		{Name: "bz"},
		{Name: "ca"},
		{Name: "cc"},
		{Name: "co.uk"},
		{Name: "com"},
	}

	if diff := cmp.Diff(expectedTLDs, tlds); diff != "" {
		t.Fatalf("Expected TLDs does not match: %s", diff)
	}

	if len(tlds) != 6 {
		t.Errorf("Expected 6 TLDs, got %d", len(tlds))
	}

	tldNames := make(map[string]bool)
	for _, tld := range tlds {
		tldNames[tld.Name] = true
	}

	expectedNames := []string{"biz", "bz", "ca", "cc", "co.uk", "com"}
	for _, name := range expectedNames {
		if !tldNames[name] {
			t.Errorf("Expected TLD %s not found in results", name)
		}
	}
}
