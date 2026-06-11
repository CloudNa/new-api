package model

import (
	"testing"

	"github.com/QuantumNous/new-api/constant"
)

func TestChannelInfoScanAcceptsStringAndBytes(t *testing.T) {
	raw := `{"is_multi_key":true,"multi_key_size":2,"multi_key_status_list":{"1":2},"multi_key_mode":"polling"}`

	for name, value := range map[string]interface{}{
		"string": raw,
		"bytes":  []byte(raw),
	} {
		t.Run(name, func(t *testing.T) {
			var info ChannelInfo
			if err := info.Scan(value); err != nil {
				t.Fatalf("Scan returned error: %v", err)
			}
			if !info.IsMultiKey {
				t.Fatal("expected multi-key flag to be preserved")
			}
			if info.MultiKeySize != 2 {
				t.Fatalf("expected multi-key size 2, got %d", info.MultiKeySize)
			}
			if info.MultiKeyMode != constant.MultiKeyModePolling {
				t.Fatalf("expected polling mode, got %q", info.MultiKeyMode)
			}
			if info.MultiKeyStatusList[1] != 2 {
				t.Fatalf("expected disabled status for key 1, got %d", info.MultiKeyStatusList[1])
			}
		})
	}
}

func TestChannelInfoScanAcceptsEmptyValues(t *testing.T) {
	for name, value := range map[string]interface{}{
		"nil":   nil,
		"empty": "",
		"blank": []byte(" \n\t "),
	} {
		t.Run(name, func(t *testing.T) {
			info := ChannelInfo{IsMultiKey: true, MultiKeySize: 3}
			if err := info.Scan(value); err != nil {
				t.Fatalf("Scan returned error: %v", err)
			}
			if info.IsMultiKey || info.MultiKeySize != 0 || info.MultiKeyStatusList != nil || info.MultiKeyMode != "" {
				t.Fatalf("expected zero ChannelInfo, got %+v", info)
			}
		})
	}
}
