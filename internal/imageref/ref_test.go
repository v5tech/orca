package imageref

import "testing"

func TestAcceleratePrepend(t *testing.T) {
	tests := map[string]string{
		"nginx":                        "m.daocloud.io/docker.io/library/nginx",
		"library/busybox":              "m.daocloud.io/docker.io/library/busybox",
		"docker.io/library/nginx:1.27": "m.daocloud.io/docker.io/library/nginx:1.27",
		"ghcr.io/foo/bar:2.0":          "m.daocloud.io/ghcr.io/foo/bar:2.0",
	}

	for in, want := range tests {
		got, reason := Accelerate(in, StrategyPrepend)
		if got != want {
			t.Errorf("Accelerate(%q) = %q, %q; want %q", in, got, reason, want)
		}
	}
}

func TestAccelerateReplaceOfficialPrefixes(t *testing.T) {
	tests := map[string]string{
		"docker.io/library/busybox:1":                 "docker.m.daocloud.io/library/busybox:1",
		"docker.elastic.co/elasticsearch/es:8.0.0":    "elastic.m.daocloud.io/elasticsearch/es:8.0.0",
		"dhi.io/foo/bar:1":                            "dhi.m.daocloud.io/foo/bar:1",
		"gcr.io/google-containers/pause:3.9":          "gcr.m.daocloud.io/google-containers/pause:3.9",
		"ghcr.io/foo/bar:1":                           "ghcr.m.daocloud.io/foo/bar:1",
		"k8s.gcr.io/coredns/coredns:2.0.0":            "k8s-gcr.m.daocloud.io/coredns/coredns:2.0.0",
		"registry.k8s.io/pause:3.9":                   "k8s.m.daocloud.io/pause:3.9",
		"mcr.microsoft.com/dotnet/runtime:8.0":        "mcr.m.daocloud.io/dotnet/runtime:8.0",
		"nvcr.io/nvidia/pytorch:24.01":                "nvcr.m.daocloud.io/nvidia/pytorch:24.01",
		"quay.io/prometheus/prometheus:v2.55.0":       "quay.m.daocloud.io/prometheus/prometheus:v2.55.0",
		"registry.ollama.ai/library/deepseek-r1:1.5b": "ollama.m.daocloud.io/library/deepseek-r1:1.5b",
	}

	for in, want := range tests {
		got, reason := Accelerate(in, StrategyReplace)
		if got != want {
			t.Errorf("Accelerate(%q) = %q, %q; want %q", in, got, reason, want)
		}
	}
}

func TestAccelerateSkips(t *testing.T) {
	skips := []string{
		"scratch",
		"docker.io/library/scratch",
		"${IMAGE}",
		"{{ .Values.image }}",
		"m.daocloud.io/docker.io/library/nginx",
		"docker.m.daocloud.io/library/nginx",
		"registry.internal.example.com/team/app:1",
	}

	for _, in := range skips {
		got, reason := Accelerate(in, StrategyPrepend)
		if got != "" {
			t.Errorf("Accelerate(%q) = %q; want empty", in, got)
		}
		if reason == "" {
			t.Errorf("Accelerate(%q) returned empty reason", in)
		}
	}
}

func TestReplaceSkipsRegistryWithoutOfficialPrefix(t *testing.T) {
	got, reason := Accelerate("public.ecr.aws/nginx:1.27", StrategyReplace)
	if got != "" {
		t.Fatalf("Accelerate() = %q; want empty", got)
	}
	if reason == "" {
		t.Fatal("Accelerate() returned empty reason")
	}
}

func TestAccelerateKeepsDigest(t *testing.T) {
	digest := "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	got, reason := Accelerate("nginx@"+digest, StrategyPrepend)
	want := "m.daocloud.io/docker.io/library/nginx@" + digest
	if got != want {
		t.Fatalf("Accelerate() = %q, %q; want %q", got, reason, want)
	}
}
