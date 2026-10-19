package bundlecontract

import "testing"

func TestViewCopiesAndRejectsDuplicateProjection(t *testing.T) {
	r := ReleaseRef{InstallID: "01234567-89ab-4def-8123-456789abcdef", BundleID: "example/experts", ReleaseDigest: "ac57d0d77f4eedfd11049d85a9f87be5c4b49d030cfd98c94a2755ea0f96a64b", ReleaseSequence: 1}
	id, _ := SkillID(r.InstallID, "plan")
	c := ComponentRecord{Ref: ComponentRef{InstallID: r.InstallID, Kind: Skill, LocalID: "plan"}, ProjectedID: id, Release: r, Body: "body", Resources: []string{"resource.md"}}
	items := []ComponentRecord{c}
	v, err := NewView(CatalogIdentity{}, []ReleaseRef{r}, items, nil)
	if err != nil {
		t.Fatal(err)
	}
	items[0].Body = "changed"
	items[0].Resources[0] = "changed"
	out := v.Components()
	if out[0].Body != "body" || out[0].Resources[0] != "resource.md" {
		t.Fatal(out)
	}
	out[0].Resources[0] = "changed"
	if v.Components()[0].Resources[0] != "resource.md" {
		t.Fatal("accessor aliases")
	}
	_, err = NewView(CatalogIdentity{}, []ReleaseRef{r}, []ComponentRecord{c, c}, nil)
	requireCode(t, err, "invalid_manifest")
}
