package runner

import (
	"reflect"
	"testing"
)

func TestMergeEnvOverride(t *testing.T) {
	base := []string{"PATH=/bin", "API_KEY=local"}
	got := MergeEnv(base, map[string]string{"API_KEY": "remote", "DB": "pg"}, false)
	want := []string{"PATH=/bin", "API_KEY=remote", "DB=pg"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMergeEnvKeepExisting(t *testing.T) {
	base := []string{"PATH=/bin", "API_KEY=local"}
	got := MergeEnv(base, map[string]string{"API_KEY": "remote", "DB": "pg"}, true)
	want := []string{"PATH=/bin", "API_KEY=local", "DB=pg"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestMergeEnvValueWithEquals(t *testing.T) {
	got := MergeEnv([]string{"X=a=b"}, map[string]string{"Y": "c=d"}, false)
	want := []string{"X=a=b", "Y=c=d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
