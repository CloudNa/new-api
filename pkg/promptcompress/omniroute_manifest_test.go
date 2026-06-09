package promptcompress

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var omniRouteExpectedBlobSHA = map[string]string{
	"omniroute/caveman_rules/_schema.json":          "9ec4fc4724de10055c4a2f652f583b4ef82aabde",
	"omniroute/caveman_rules/de/context.json":       "64173166bf715ed125830cc19e16de847ee5c589",
	"omniroute/caveman_rules/de/filler.json":        "f4f2e6eb62215a8547778001d9b1ca0f03d32cc7",
	"omniroute/caveman_rules/de/structural.json":    "7107c977b38c1b4db85f3ce17bcc3cd20c2bb596",
	"omniroute/caveman_rules/en/context.json":       "70ef1484087342a56f1de7ae6ddacae387cc0ddd",
	"omniroute/caveman_rules/en/dedup.json":         "23f0820bfe77863851156eeee00fa588004cf7fc",
	"omniroute/caveman_rules/en/filler.json":        "88e4a6978a6826d41c2c5da46093a6b6a9740ab3",
	"omniroute/caveman_rules/en/structural.json":    "b71f2c64bc34cb8e2c768fcb4a6e303613557797",
	"omniroute/caveman_rules/en/ultra.json":         "1e085d7607f857b6664a5ee41011817e4edea8d4",
	"omniroute/caveman_rules/es/context.json":       "0ef5a8ed333c5f6856f8dc1773f36ac396b6f12a",
	"omniroute/caveman_rules/es/dedup.json":         "ac5e52239dd9274bc820c8e9577aee00bc7d74c0",
	"omniroute/caveman_rules/es/filler.json":        "cf612f1ce090431f0e751b9ecd5a8be745d56fb9",
	"omniroute/caveman_rules/es/structural.json":    "759ac33d4beee3ce546cbc96624272b1575e3c01",
	"omniroute/caveman_rules/es/ultra.json":         "bbc51cd0c02da63397bf7b4d72a7813ed48e626b",
	"omniroute/caveman_rules/fr/context.json":       "78721d4fb21ae608b9655b504954b0252c781cd0",
	"omniroute/caveman_rules/fr/filler.json":        "61a2caa40fc011e46ec2db9655d0e4b9c6146e76",
	"omniroute/caveman_rules/fr/structural.json":    "f59ac1d05d063256f61977739e3fcfb948688989",
	"omniroute/caveman_rules/ja/context.json":       "e081dd8e59fc8bceadd83c65ed9300bc1e9cde50",
	"omniroute/caveman_rules/ja/filler.json":        "c7143b3f3d70b35a59d6ba8f0828a551cd1ad7ef",
	"omniroute/caveman_rules/ja/structural.json":    "41c650c97bb3359f8fcbf2ae277d80e81b15840e",
	"omniroute/caveman_rules/pt-BR/context.json":    "90ed2d58f087faba46a081ddc7b1322535011315",
	"omniroute/caveman_rules/pt-BR/dedup.json":      "fa1c0d08aa003707689ed5dc70e6f343a3b5d1ee",
	"omniroute/caveman_rules/pt-BR/filler.json":     "6763e9bf6b051deaba46e5dd18489d7df28906a7",
	"omniroute/caveman_rules/pt-BR/structural.json": "72090247649011cd85bd8668ca7da6c6ac880a01",
	"omniroute/caveman_rules/pt-BR/ultra.json":      "606cc22457d857bac0bc50567dbaca30e30b52c0",
	"omniroute/rtk_filters/aws.json":                "7a3f82d887ff96f9208aa3e17e7e4ac94d9107c4",
	"omniroute/rtk_filters/biome.json":              "d0c7564b3999018d3321c782903af6096dbf7d10",
	"omniroute/rtk_filters/build-eslint.json":       "610e6163635140284d6858749953c0acaa38ae05",
	"omniroute/rtk_filters/build-typescript.json":   "5ef3e314c5e933ce273fad2f77ef941a16bf38f5",
	"omniroute/rtk_filters/build-vite.json":         "7e81845b81d4df4a19df1509ba9353235deb6573",
	"omniroute/rtk_filters/build-webpack.json":      "55e1eb845bd36c9c0801e20742105ce4a884e78b",
	"omniroute/rtk_filters/bundle-install.json":     "7b126ae117036cbef4b6f77217b0ec0a132d6980",
	"omniroute/rtk_filters/composer.json":           "1391699634bcc0ee5d304bed077759a7f02da356",
	"omniroute/rtk_filters/curl.json":               "936f290706346e718e866f423bc8064e6e5278b9",
	"omniroute/rtk_filters/df.json":                 "5be798a2735419330d19276da2671c648793ef56",
	"omniroute/rtk_filters/docker-build.json":       "efdf5452cf102a680e048fb7179b9d8f192c9f24",
	"omniroute/rtk_filters/docker-logs.json":        "10b5d11f43b48ca1ed93b29c542319f98ec960c4",
	"omniroute/rtk_filters/docker-ps.json":          "b7dc4efee0a5a8677c9d9b880c90060be9885085",
	"omniroute/rtk_filters/du.json":                 "54cca64e89da9b89addf908846e4b76656009a0c",
	"omniroute/rtk_filters/error-stacktrace.json":   "0d5fd3b8bffd5b075e0e43ab20ef9df8bba63fd9",
	"omniroute/rtk_filters/gcloud.json":             "19c5dd09a9b5b789d428bcec78d9997e47dbd8b6",
	"omniroute/rtk_filters/generic-output.json":     "da93022888913ab9495d3bc42d1f509a1bfa80e2",
	"omniroute/rtk_filters/gh.json":                 "42fe6e9865016b6b17db149e02f04a6d28fef4ee",
	"omniroute/rtk_filters/git-branch.json":         "b333932f08a78af36f55ce8e900ddad6fef0197c",
	"omniroute/rtk_filters/git-diff.json":           "a97df1fbacc5eb6526a22233f6eef036ed3f2983",
	"omniroute/rtk_filters/git-log.json":            "4a541605379677eee924060192d4081b41d4d87b",
	"omniroute/rtk_filters/git-status.json":         "d2d21a66f589941c930ce933a6960372f9988bea",
	"omniroute/rtk_filters/golangci-lint.json":      "8bffbee1afe0caf87376d3a1fc4de3c5964f8e96",
	"omniroute/rtk_filters/json-output.json":        "1ab8769266f0024fe3553c4145d4be52cef5e115",
	"omniroute/rtk_filters/kubectl.json":            "eeb9c5bea40e85ac7a5541bcdf90fa32473ab25b",
	"omniroute/rtk_filters/make.json":               "7b9d3a6b241659399eb5716d59aefb8cda43de78",
	"omniroute/rtk_filters/mypy.json":               "861bbe3e9b4e7e1c07a327fbc44e868134cf273c",
	"omniroute/rtk_filters/npm-audit.json":          "19fa224a38be9fd05731592f6456e13c058ae2a4",
	"omniroute/rtk_filters/npm-install.json":        "df001e67b2b811a2337dad4d92679123f4fbf0c0",
	"omniroute/rtk_filters/nx.json":                 "6973433228d1fb3c45f49ebefd04ef10ccd39e89",
	"omniroute/rtk_filters/pip.json":                "91edf38ca3ba408979babc565edb2188b1a55389",
	"omniroute/rtk_filters/playwright.json":         "1899b6367c9c69466a0f3d8b162a5ead172bf513",
	"omniroute/rtk_filters/poetry-install.json":     "f40169365e275cfb5709004f83ee91219cd1e9d6",
	"omniroute/rtk_filters/prettier.json":           "0a1c894fc6f247e86c954cbe58d3f87398cdc050",
	"omniroute/rtk_filters/ps.json":                 "843866d70eaef524f7d6e61e279f18272993843c",
	"omniroute/rtk_filters/rsync.json":              "36ecd135bc7bdcd32ca62312ebc593bb5125935d",
	"omniroute/rtk_filters/rubocop.json":            "da92cc4bebab7118b2843d6a893b0c0cc06db9ca",
	"omniroute/rtk_filters/ruff.json":               "0e3d80d6eea8c588d87df1daa8a217dbac14eeba",
	"omniroute/rtk_filters/shell-find.json":         "e95ce818d943959e8fe41059c3a60b0accdef1cc",
	"omniroute/rtk_filters/shell-grep.json":         "6429ff82f984f969e0d54ae8804a5ea040bc095f",
	"omniroute/rtk_filters/shell-ls.json":           "d8bb51cf8ebc832c8690ac9c0520ed90419b5e04",
	"omniroute/rtk_filters/ssh.json":                "2b6775977689d0e458b2ed6f1d8a46a6e01c123a",
	"omniroute/rtk_filters/systemctl-status.json":   "f75b502bc1ec1529e5f36842512456e0de13da3f",
	"omniroute/rtk_filters/terraform-plan.json":     "3f9d417253783939984bdf0bac83a855fe0a066a",
	"omniroute/rtk_filters/test-cargo.json":         "e7aecaca5c239a6c7ff93cfebf73079bcc6f6bd1",
	"omniroute/rtk_filters/test-go.json":            "5e4f05e50dd45ff9c21c4c701904829169804826",
	"omniroute/rtk_filters/test-jest.json":          "62f4c0a5d056f2cc8c0cc351cc1e7c9bce5e48b9",
	"omniroute/rtk_filters/test-pytest.json":        "4e62bcc34cc83b1fc5b87794d78e93fb36d2f64b",
	"omniroute/rtk_filters/test-vitest.json":        "bb8a7480de4bdb12da954dfd64b32cf38d568a18",
	"omniroute/rtk_filters/tofu-plan.json":          "d6b8c69ee52db8dd0e241c5cea5724949aa6dc78",
	"omniroute/rtk_filters/turbo.json":              "0727717b65ef617127c65825cd4d281225ceda9a",
	"omniroute/rtk_filters/uv-sync.json":            "ecac0a032171b52349c807f8174286cb34cd4afd",
	"omniroute/rtk_filters/wget.json":               "b5de52bb992bd98cfcb8018dde68616ebae053bc",
}

func TestOmniRouteVendoredRuleBlobParity(t *testing.T) {
	require.Equal(t, "630baa6", OmniRouteParityCommit)
	seen := map[string]bool{}
	err := fs.WalkDir(omniRouteFS, "omniroute", func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(filePath, ".json") {
			return nil
		}
		expected, ok := omniRouteExpectedBlobSHA[filePath]
		require.Truef(t, ok, "unexpected vendored OmniRoute file %s", filePath)
		data, err := omniRouteFS.ReadFile(filePath)
		require.NoError(t, err)
		require.Equalf(t, expected, gitBlobSHA(data), "vendored OmniRoute file drifted: %s", filePath)
		seen[filePath] = true
		return nil
	})
	require.NoError(t, err)
	for filePath := range omniRouteExpectedBlobSHA {
		require.Truef(t, seen[filePath], "missing vendored OmniRoute file %s", filePath)
	}
}

func gitBlobSHA(data []byte) string {
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	h := sha1.New()
	_, _ = fmt.Fprintf(h, "blob %d\x00", len(data))
	_, _ = h.Write(data)
	return fmt.Sprintf("%x", h.Sum(nil))
}
