package protocol

type AssurancePolicy struct {
	AllowedTrustCategories []string
	ProofSupport           string
}

type CheckerProfile struct {
	Name    string
	Version string
}

type Limit struct {
	Name  string
	Unit  string
	Value uint64
}

type Request struct {
	AssurancePolicy AssurancePolicy
	BundleManifest  Digest
	CheckerProfile  CheckerProfile
	Evidence        Digest
	Limits          []Limit
	Version         string
}

func (r Request) Limit(name string) (uint64, bool) {
	for _, limit := range r.Limits {
		if limit.Name == name {
			return limit.Value, true
		}
	}
	return 0, false
}

type Artifact struct {
	ByteLength    uint64
	ContentDigest Digest
	Format        string
	FormatVersion string
	Roles         []string
}

func (a Artifact) HasRole(want string) bool {
	for _, role := range a.Roles {
		if role == want {
			return true
		}
	}
	return false
}

type Manifest struct {
	Artifacts []Artifact
	Version   string
}
