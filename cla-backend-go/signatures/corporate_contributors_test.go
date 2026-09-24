// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"context"
	"fmt"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/golang/mock/gomock"
	mock_company "github.com/linuxfoundation/easycla/cla-backend-go/company/mocks"
	eventsMock "github.com/linuxfoundation/easycla/cla-backend-go/events/mock"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	mock_users "github.com/linuxfoundation/easycla/cla-backend-go/users/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fakeFalse() map[string]interface{} {
	return map[string]interface{}{"BOOL": false}
}

// corporateContributorItems is a company's acknowledgment table as the DynamoDB stream leaves it:
// an approved one, one invalidated with the M2 attribution attributes, one invalidated the pre-M2
// way (note only), an unsigned one, and a second company's record under the same CLA group
func corporateContributorItems() []map[string]interface{} {
	approved := fakeEclaItem(1, "approved.dev@example.com")
	approved["user_lf_username"] = fakeS("approved-dev")

	invalidated := fakeEclaItem(2, "removed.dev@example.com")
	invalidated["signature_approved"] = fakeFalse()
	invalidated["note"] = fakeS("Signature invalidated (approved set to false) due to approval list removal")
	invalidated["date_invalidated"] = fakeS("2026-09-01T10:11:12.123456+0000")
	invalidated["invalidated_by"] = fakeS("cla-manager")
	invalidated["invalidation_reason"] = fakeS("approved list removal (EmailApprovalList)")
	invalidated["invalidation_note"] = fakeS("left the company")

	legacyInvalidated := fakeEclaItem(3, "legacy.dev@example.com")
	legacyInvalidated["signature_approved"] = fakeFalse()
	legacyInvalidated["note"] = fakeS("Signature invalidated (approved set to false) by pcc-admin for legacy-dev ")

	unsigned := fakeEclaItem(4, "unsigned.dev@example.com")
	unsigned["signature_signed"] = fakeFalse()

	otherCompany := fakeEclaItem(5, "other.dev@example.com")
	otherCompany["signature_user_ccla_company_id"] = fakeS("company-2")

	return []map[string]interface{}{approved, invalidated, legacyInvalidated, unsigned, otherCompany}
}

func newCorporateContributorsRepo(t *testing.T, items []map[string]interface{}) (repository, *fakeSignaturesTable) {
	t.Helper()
	table := &fakeSignaturesTable{items: items, invalidated: map[string]int{}}
	server := httptest.NewServer(table)
	t.Cleanup(server.Close)

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("us-east-1"),
		Endpoint:    aws.String(server.URL),
		Credentials: credentials.NewStaticCredentials("test", "test", ""),
		DisableSSL:  aws.Bool(true),
	})
	require.NoError(t, err)

	ctrl := gomock.NewController(t)
	mockUsers := mock_users.NewMockUserRepository(ctrl)
	mockUsers.EXPECT().GetUser(gomock.Any()).AnyTimes().DoAndReturn(func(userID string) (*models.User, error) {
		return &models.User{UserID: userID, Username: "user " + userID}, nil
	})
	mockCompany := mock_company.NewMockIRepository(ctrl)
	mockCompany.EXPECT().GetCompany(gomock.Any(), gomock.Any()).AnyTimes().DoAndReturn(func(_ context.Context, companyID string) (*models.Company, error) {
		return &models.Company{CompanyID: companyID, CompanyName: "company " + companyID}, nil
	})
	mockEvents := eventsMock.NewMockService(ctrl)
	mockEvents.EXPECT().LogEvent(gomock.Any()).AnyTimes()
	mockEvents.EXPECT().LogEventWithContext(gomock.Any(), gomock.Any()).AnyTimes()

	return repository{
		stage:              "test",
		dynamoDBClient:     dynamodb.New(sess),
		companyRepo:        mockCompany,
		usersRepo:          mockUsers,
		eventsService:      mockEvents,
		signatureTableName: "cla-test-signatures",
	}, table
}

func TestGetClaGroupCorporateContributorsRowSetMatchesTotal(t *testing.T) {
	repo, _ := newCorporateContributorsRepo(t, corporateContributorItems())

	result, err := repo.GetClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-1"), aws.Int64(10), nil, nil)
	require.NoError(t, err)

	ids := make([]string, 0, len(result.List))
	for _, row := range result.List {
		ids = append(ids, row.SignatureID)
	}
	sort.Strings(ids)
	assert.Equal(t, []string{"sig-001", "sig-002", "sig-003"}, ids, "signed acknowledgments of the company, invalidated ones included, unsigned and other companies excluded")
	assert.Equal(t, int64(3), result.TotalCount, "the total counts exactly the returned row set")
	assert.Equal(t, int64(3), result.ResultCount)
	assert.Equal(t, "", result.NextKey, "a first page holding every row has no continuation, even with filtered-out items left in the key range")

	rows := map[string]*models.CorporateContributor{}
	for _, row := range result.List {
		rows[row.SignatureID] = row
	}

	approved := rows["sig-001"]
	assert.True(t, approved.SignatureApproved)
	assert.True(t, approved.SignatureSigned)
	assert.Equal(t, "approved-dev", approved.LinuxFoundationID)
	assert.Equal(t, "approved.dev@example.com", approved.Email)
	assert.Equal(t, "", approved.InvalidatedAt)
	assert.Equal(t, "", approved.InvalidatedBy)
	assert.Equal(t, "", approved.InvalidationReason)
	assert.Equal(t, "", approved.InvalidationNote)
	assert.Equal(t, "", approved.Note)

	invalidated := rows["sig-002"]
	assert.False(t, invalidated.SignatureApproved)
	assert.True(t, invalidated.SignatureSigned)
	assert.Equal(t, "2026-09-01T10:11:12Z", invalidated.InvalidatedAt, "stored date_invalidated, normalized")
	assert.Equal(t, "cla-manager", invalidated.InvalidatedBy)
	assert.Equal(t, "approved list removal (EmailApprovalList)", invalidated.InvalidationReason)
	assert.Equal(t, "left the company", invalidated.InvalidationNote)
	assert.Equal(t, "Signature invalidated (approved set to false) due to approval list removal", invalidated.Note)

	legacy := rows["sig-003"]
	assert.False(t, legacy.SignatureApproved)
	assert.Equal(t, "", legacy.InvalidatedAt, "pre-M2 invalidations carry no attribution attributes")
	assert.Equal(t, "", legacy.InvalidatedBy)
	assert.Equal(t, "Signature invalidated (approved set to false) by pcc-admin for legacy-dev ", legacy.Note, "the note is the only trace left by a pre-M2 invalidation")
}

func TestGetClaGroupCorporateContributorsPaging(t *testing.T) {
	repo, _ := newCorporateContributorsRepo(t, corporateContributorItems())

	first, err := repo.GetClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-1"), aws.Int64(2), nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(3), first.TotalCount, "the total is independent of the page size")
	assert.Equal(t, int64(2), first.ResultCount)
	assert.NotEmpty(t, first.NextKey)

	second, err := repo.GetClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-1"), aws.Int64(2), aws.String(first.NextKey), nil)
	require.NoError(t, err)
	assert.Equal(t, int64(3), second.TotalCount)
	assert.Equal(t, int64(1), second.ResultCount)
	assert.Equal(t, "", second.NextKey, "the key range is exhausted")

	seen := map[string]bool{}
	for _, row := range append(first.List, second.List...) {
		seen[row.SignatureID] = true
	}
	assert.Equal(t, map[string]bool{"sig-001": true, "sig-002": true, "sig-003": true}, seen, "both pages together are exactly the row set")
}

func TestGetClaGroupCorporateContributorsNoRows(t *testing.T) {
	repo, _ := newCorporateContributorsRepo(t, corporateContributorItems())

	result, err := repo.GetClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-3"), nil, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.TotalCount)
	assert.Equal(t, int64(0), result.ResultCount)
	assert.NotNil(t, result.List)
	assert.Len(t, result.List, 0)
}

func TestGetClaGroupCorporateContributorsIdentityFallsBackToTheUserRecord(t *testing.T) {
	bare := fakeEclaItem(1, "bare.dev@example.com")
	stored := fakeEclaItem(2, "stored.dev@example.com")
	stored["user_name"] = fakeS("Stored Name")
	stored["user_lf_username"] = fakeS("stored-lf")
	stored[SignatureUserGitHubUsername] = fakeS("stored-gh")
	stored[SignatureUserGitlabUsername] = fakeS("stored-gl")
	partial := fakeEclaItem(3, "partial.dev@example.com")
	partial[SignatureUserGitHubUsername] = fakeS("partial-gh")
	orphan := fakeEclaItem(4, "orphan.dev@example.com")
	failing := fakeEclaItem(5, "failing.dev@example.com")

	users := map[string]*models.User{
		"user-001": {UserID: "user-001", Username: "Bare User", LfUsername: "bare-lf", GithubUsername: "bare-gh", GitlabUsername: "bare-gl"},
		"user-002": {UserID: "user-002", Username: "Other Name", LfUsername: "other-lf", GithubUsername: "other-gh", GitlabUsername: "other-gl"},
		"user-003": {UserID: "user-003", Username: "Partial User", LfUsername: "partial-lf", GithubUsername: "other-gh", GitlabUsername: "partial-gl"},
	}
	repo, _ := newCorporateContributorsRepo(t, []map[string]interface{}{bare, stored, partial, orphan, failing})
	mockUsers := mock_users.NewMockUserRepository(gomock.NewController(t))
	mockUsers.EXPECT().GetUser(gomock.Any()).AnyTimes().DoAndReturn(func(userID string) (*models.User, error) {
		if userID == "user-005" {
			return nil, fmt.Errorf("user lookup failed")
		}
		return users[userID], nil
	})
	repo.usersRepo = mockUsers

	result, err := repo.GetClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-1"), aws.Int64(10), nil, nil)
	require.NoError(t, err)
	require.Equal(t, int64(5), result.ResultCount)
	rows := map[string]*models.CorporateContributor{}
	for _, row := range result.List {
		rows[row.SignatureID] = row
	}

	cases := []struct {
		name        string
		signatureID string
		want        models.CorporateContributor
	}{
		{"identities missing on the signature come from the user record", "sig-001",
			models.CorporateContributor{Name: "Bare User", LinuxFoundationID: "bare-lf", GithubID: "bare-gh", GitlabID: "bare-gl", Email: "bare.dev@example.com"}},
		{"identities stored on the signature win", "sig-002",
			models.CorporateContributor{Name: "Stored Name", LinuxFoundationID: "stored-lf", GithubID: "stored-gh", GitlabID: "stored-gl", Email: "stored.dev@example.com"}},
		{"each identity falls back on its own", "sig-003",
			models.CorporateContributor{Name: "Partial User", LinuxFoundationID: "partial-lf", GithubID: "partial-gh", GitlabID: "partial-gl", Email: "partial.dev@example.com"}},
		{"a missing user record leaves the signature values", "sig-004",
			models.CorporateContributor{Email: "orphan.dev@example.com"}},
		{"a failing user lookup leaves the signature values", "sig-005",
			models.CorporateContributor{Email: "failing.dev@example.com"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := rows[tc.signatureID]
			require.NotNil(t, row)
			assert.Equal(t, tc.want.Name, row.Name)
			assert.Equal(t, tc.want.LinuxFoundationID, row.LinuxFoundationID)
			assert.Equal(t, tc.want.GithubID, row.GithubID)
			assert.Equal(t, tc.want.GitlabID, row.GitlabID)
			assert.Equal(t, tc.want.Email, row.Email, "the email is not backfilled")
			assert.True(t, row.SignatureApproved)
			assert.True(t, row.SignatureSigned)
		})
	}
}

func TestCountClaGroupCorporateContributors(t *testing.T) {
	repo, table := newCorporateContributorsRepo(t, corporateContributorItems())

	all, err := repo.CountClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-1"), false, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(3), all, "list total: signed acknowledgments in every approval state")

	approvedOnly, err := repo.CountClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-1"), true, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), approvedOnly, "approvedContributorsCount: approved and signed only")

	other, err := repo.CountClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-2"), false, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), other)

	for _, query := range table.queries {
		assert.Equal(t, SignatureProjectIDIndex, query.indexName)
		assert.Equal(t, int64(HugePageSize), query.limit, "the count walks the whole index partition")
	}
}

func resolvedCorporateContributorFilter(t *testing.T, companyID *string, approvedOnly bool) string {
	t.Helper()
	filter := corporateContributorFilter(companyID, approvedOnly)
	expr, err := expression.NewBuilder().WithFilter(filter).Build()
	require.NoError(t, err)

	resolved := aws.StringValue(expr.Filter())
	for placeholder, name := range expr.Names() {
		resolved = strings.ReplaceAll(resolved, placeholder, aws.StringValue(name))
	}
	for placeholder, value := range expr.Values() {
		switch {
		case value.S != nil:
			resolved = strings.ReplaceAll(resolved, placeholder, *value.S)
		case value.BOOL != nil:
			resolved = strings.ReplaceAll(resolved, placeholder, strconv.FormatBool(*value.BOOL))
		}
	}
	return resolved
}

func TestCorporateContributorFilterShape(t *testing.T) {
	t.Run("company and signed only", func(t *testing.T) {
		resolved := resolvedCorporateContributorFilter(t, aws.String("company-1"), false)
		assert.Equal(t, "(signature_user_ccla_company_id = company-1) AND (signature_signed = true)", resolved)
	})

	t.Run("approved only adds the approval condition", func(t *testing.T) {
		resolved := resolvedCorporateContributorFilter(t, aws.String("company-1"), true)
		assert.Equal(t, "((signature_user_ccla_company_id = company-1) AND (signature_signed = true)) AND (signature_approved = true)", resolved)
	})

	t.Run("search term never reaches DynamoDB", func(t *testing.T) {
		resolved := resolvedCorporateContributorFilter(t, aws.String("company-1"), false)
		assert.NotContains(t, resolved, "contains", "matching is done in memory, case-insensitively")
	})
}

func TestCorporateContributorSearchTerm(t *testing.T) {
	assert.Equal(t, "", corporateContributorSearchTerm(nil))
	assert.Equal(t, "", corporateContributorSearchTerm(aws.String("   ")))
	assert.Equal(t, "octocat", corporateContributorSearchTerm(aws.String(" OctoCat ")))
}

func TestCorporateContributorMatches(t *testing.T) {
	sig := &ItemSignature{
		SignatureReferenceNameLower: "jane doe",
		UserName:                    "Jane Doe",
		UserEmail:                   "Jane.Doe@Example.com",
		UserLFUsername:              "jdoe",
		UserGithubUsername:          "JaneGH",
		UserGitlabUsername:          "JaneGL",
		UserDocusignName:            "Jane D.",
	}
	for _, term := range []string{"JaneGH", "janegh", "JANEGH", "janegl", "jane doe", "jane.doe@", "JDOE", "jane d.", "example"} {
		assert.True(t, corporateContributorMatches(sig, corporateContributorSearchTerm(aws.String(term))), "term %q", term)
	}
	assert.False(t, corporateContributorMatches(sig, "octocat"))
	assert.True(t, corporateContributorMatches(sig, ""), "no term matches everything")
	assert.False(t, corporateContributorMatches(&ItemSignature{}, "jane"), "empty attributes never match")
	assert.True(t, corporateContributorMatches(&ItemSignature{UserGithubUsername: "OnlyLogin"}, "onlylogin"), "a login-only record is found by its login")
}

func mixedCaseContributorItems() []map[string]interface{} {
	items := corporateContributorItems()
	login := fakeEclaItem(6, "jane.doe@example.com")
	login["user_name"] = fakeS("Jane Doe")
	login["signature_reference_name_lower"] = fakeS("jane doe")
	login["user_github_username"] = fakeS("JaneGH")
	return append(items, login)
}

func TestGetClaGroupCorporateContributorsSearchIsCaseInsensitive(t *testing.T) {
	for _, term := range []string{"JaneGH", "janegh", "JANEGH", " JaneGh "} {
		t.Run(term, func(t *testing.T) {
			repo, table := newCorporateContributorsRepo(t, mixedCaseContributorItems())

			result, err := repo.GetClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-1"), aws.Int64(10), nil, aws.String(term))
			require.NoError(t, err)
			require.Len(t, result.List, 1, "the exact login and every case variant find the record")
			assert.Equal(t, "sig-006", result.List[0].SignatureID)
			assert.Equal(t, "JaneGH", result.List[0].GithubID, "stored case is preserved")
			assert.Equal(t, int64(1), result.TotalCount, "the count uses the same matcher")
			assert.Equal(t, int64(1), result.ResultCount)
			assert.Equal(t, "", result.NextKey)

			for _, query := range table.queries {
				assert.Equal(t, int64(HugePageSize), query.limit, "a search evaluates the company's rows in big windows")
			}
		})
	}

	t.Run("no match", func(t *testing.T) {
		repo, _ := newCorporateContributorsRepo(t, mixedCaseContributorItems())
		result, err := repo.GetClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-1"), aws.Int64(10), nil, aws.String("octocat"))
		require.NoError(t, err)
		assert.Equal(t, int64(0), result.TotalCount)
		assert.Len(t, result.List, 0)
	})

	t.Run("other companies and unsigned rows stay out of a search", func(t *testing.T) {
		repo, _ := newCorporateContributorsRepo(t, mixedCaseContributorItems())
		result, err := repo.GetClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-1"), aws.Int64(10), nil, aws.String("dev@example.com"))
		require.NoError(t, err)
		ids := make([]string, 0, len(result.List))
		for _, row := range result.List {
			ids = append(ids, row.SignatureID)
		}
		sort.Strings(ids)
		assert.Equal(t, []string{"sig-001", "sig-002", "sig-003"}, ids, "unsigned sig-004 and company-2's sig-005 match the term but are outside the row set")
		assert.Equal(t, int64(3), result.TotalCount)
	})

	t.Run("search pages", func(t *testing.T) {
		repo, _ := newCorporateContributorsRepo(t, mixedCaseContributorItems())
		first, err := repo.GetClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-1"), aws.Int64(2), nil, aws.String("EXAMPLE.COM"))
		require.NoError(t, err)
		assert.Equal(t, int64(4), first.TotalCount)
		assert.Equal(t, int64(2), first.ResultCount)
		require.NotEmpty(t, first.NextKey)

		second, err := repo.GetClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-1"), aws.Int64(2), aws.String(first.NextKey), aws.String("EXAMPLE.COM"))
		require.NoError(t, err)
		assert.Equal(t, int64(2), second.ResultCount)
		assert.Equal(t, "", second.NextKey)

		seen := map[string]bool{}
		for _, row := range append(first.List, second.List...) {
			seen[row.SignatureID] = true
		}
		assert.Equal(t, map[string]bool{"sig-001": true, "sig-002": true, "sig-003": true, "sig-006": true}, seen)
	})
}

func TestCountClaGroupCorporateContributorsWithSearchTerm(t *testing.T) {
	repo, table := newCorporateContributorsRepo(t, mixedCaseContributorItems())

	count, err := repo.CountClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-1"), false, aws.String("janegh"))
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	approvedOnly, err := repo.CountClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-1"), true, aws.String("Example.COM"))
	require.NoError(t, err)
	assert.Equal(t, int64(2), approvedOnly, "approved + signed rows matching the term: sig-001 and sig-006")

	require.NotEmpty(t, table.queries)
	for _, name := range []string{"signature_reference_name_lower", "user_email", "user_lf_username", "user_name", SignatureUserGitHubUsername, SignatureUserGitlabUsername, "user_docusign_name"} {
		assert.Contains(t, table.queries[0].attributeNames, name, "a search-narrowed count projects the attribute it matches on")
	}
}

func TestFormatStoredTime(t *testing.T) {
	assert.Equal(t, "", formatStoredTime(""))
	assert.Equal(t, "2026-09-01T10:11:12Z", formatStoredTime("2026-09-01T10:11:12.123456+0000"))
	assert.Equal(t, "2026-09-01T10:11:12Z", formatStoredTime("2026-09-01T10:11:12Z"))
}

// boundaryContributorItems is a CLA group with 40 signatures in key order; company-1's signed
// acknowledgments sit at the given positions, the rest are another company's rows or unsigned
func boundaryContributorItems() ([]map[string]interface{}, []string) {
	matching := map[int]bool{1: true, 3: true, 4: true, 9: true, 10: true, 11: true, 12: true, 20: true, 27: true, 28: true, 29: true, 30: true, 31: true, 40: true}
	items := make([]map[string]interface{}, 0, 40)
	var expected []string
	for i := 1; i <= 40; i++ {
		item := fakeEclaItem(i, fmt.Sprintf("dev%03d@example.com", i))
		switch {
		case matching[i]:
			if i%2 == 0 {
				item["signature_type"] = fakeS("ecla")
			}
			expected = append(expected, fmt.Sprintf("sig-%03d", i))
		case i%5 == 0:
			item["signature_signed"] = fakeFalse()
		default:
			item["signature_user_ccla_company_id"] = fakeS("company-2")
		}
		items = append(items, item)
	}
	return items, expected
}

func collectContributorPages(t *testing.T, repo repository, pageSize *int64, searchTerm *string) ([]*models.CorporateContributorList, []string) {
	t.Helper()
	var pages []*models.CorporateContributorList
	var ids []string
	var nextKey *string
	for {
		page, err := repo.GetClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-1"), pageSize, nextKey, searchTerm)
		require.NoError(t, err)
		pages = append(pages, page)
		for _, row := range page.List {
			ids = append(ids, row.SignatureID)
		}
		if page.NextKey == "" {
			return pages, ids
		}
		require.Less(t, len(pages), 50, "paging must terminate")
		nextKey = aws.String(page.NextKey)
	}
}

func TestGetClaGroupCorporateContributorsAcrossRawPageBoundaries(t *testing.T) {
	cases := []struct {
		name       string
		pageSize   *int64
		wantSizes  []int64
		maxRawPage int
	}{
		{"default page size", nil, []int64{10, 4}, 3},
		{"page ends inside a later raw page", aws.Int64(4), []int64{4, 4, 4, 2}, 3},
		{"page boundary on a raw page boundary", aws.Int64(3), []int64{3, 3, 3, 3, 2}, 3},
		{"single row pages", aws.Int64(1), []int64{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}, 3},
		{"no raw page cap", aws.Int64(6), []int64{6, 6, 2}, 0},
		{"raw pages of one row", aws.Int64(5), []int64{5, 5, 4}, 1},
	}
	items, expected := boundaryContributorItems()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo, table := newCorporateContributorsRepo(t, items)
			table.maxRawPage = tc.maxRawPage

			pages, ids := collectContributorPages(t, repo, tc.pageSize, nil)

			sorted := append([]string(nil), ids...)
			sort.Strings(sorted)
			assert.Equal(t, expected, sorted, "every matching row exactly once - cla and ecla types alike")
			var sizes []int64
			for _, page := range pages {
				sizes = append(sizes, page.ResultCount)
				assert.Equal(t, int64(len(page.List)), page.ResultCount)
				assert.Equal(t, int64(len(expected)), page.TotalCount)
			}
			assert.Equal(t, tc.wantSizes, sizes)
			assert.Equal(t, "", pages[len(pages)-1].NextKey)
		})
	}
}

func TestGetClaGroupCorporateContributorsPageAfterTheLastMatchIsEmpty(t *testing.T) {
	items, _ := boundaryContributorItems()
	for i := 41; i <= 45; i++ {
		trailing := fakeEclaItem(i, fmt.Sprintf("dev%03d@example.com", i))
		trailing["signature_user_ccla_company_id"] = fakeS("company-2")
		items = append(items, trailing)
	}
	repo, table := newCorporateContributorsRepo(t, items)
	table.maxRawPage = 3

	// 14 matches, pages of 7: the second page fills up with only another company's rows still
	// ahead in the key range, so a continuation is handed out and the third page comes back empty
	pages, ids := collectContributorPages(t, repo, aws.Int64(7), nil)
	require.Len(t, pages, 3)
	assert.Equal(t, int64(7), pages[0].ResultCount)
	assert.Equal(t, int64(7), pages[1].ResultCount)
	assert.NotEmpty(t, pages[1].NextKey)
	assert.Equal(t, int64(0), pages[2].ResultCount)
	assert.Equal(t, "", pages[2].NextKey)
	assert.Len(t, ids, 14)
}

func TestCountClaGroupCorporateContributorsAcrossRawPageBoundaries(t *testing.T) {
	items, expected := boundaryContributorItems()
	repo, table := newCorporateContributorsRepo(t, items)
	table.maxRawPage = 3

	count, err := repo.CountClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-1"), false, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(len(expected)), count)
	assert.Len(t, table.queries, 14, "40 rows in windows of 3")

	searched, err := repo.CountClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-1"), false, aws.String("DEV03"))
	require.NoError(t, err)
	assert.Equal(t, int64(2), searched, "sig-030 and sig-031 match dev030/dev031")
}

func TestGetClaGroupCorporateContributorsSearchAcrossRawPageBoundaries(t *testing.T) {
	items, _ := boundaryContributorItems()
	repo, table := newCorporateContributorsRepo(t, items)
	table.maxRawPage = 3

	pages, ids := collectContributorPages(t, repo, aws.Int64(2), aws.String("dev0"))
	sort.Strings(ids)
	assert.Equal(t, []string{"sig-001", "sig-003", "sig-004", "sig-009", "sig-010", "sig-011", "sig-012", "sig-020", "sig-027", "sig-028", "sig-029", "sig-030", "sig-031", "sig-040"}, ids)
	for _, page := range pages {
		assert.Equal(t, int64(14), page.TotalCount)
	}
	assert.Equal(t, "", pages[len(pages)-1].NextKey)

	_, ids = collectContributorPages(t, repo, aws.Int64(2), aws.String("dev02"))
	sort.Strings(ids)
	assert.Equal(t, []string{"sig-020", "sig-027", "sig-028", "sig-029"}, ids)
}

func TestGetClaGroupCorporateContributorsSearchMatchesEachFieldInIsolation(t *testing.T) {
	fields := []string{"signature_reference_name_lower", "user_email", "user_lf_username", "user_name", SignatureUserGitHubUsername, SignatureUserGitlabUsername, "user_docusign_name"}
	items := corporateContributorItems()
	for i, field := range fields {
		item := fakeEclaItem(20+i, "")
		delete(item, "user_email")
		item[field] = fakeS(fmt.Sprintf("Needle-%s", field))
		items = append(items, item)
	}
	repo, _ := newCorporateContributorsRepo(t, items)

	for i, field := range fields {
		t.Run(field, func(t *testing.T) {
			result, err := repo.GetClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-1"), nil, nil, aws.String("needle-"+field))
			require.NoError(t, err)
			require.Len(t, result.List, 1, "only the row carrying the term in this field matches")
			assert.Equal(t, fmt.Sprintf("sig-%03d", 20+i), result.List[0].SignatureID)
			assert.Equal(t, int64(1), result.TotalCount)
			assert.Equal(t, "", result.NextKey)
		})
	}

	all, err := repo.GetClaGroupCorporateContributors(context.Background(), "cla-group-1", aws.String("company-1"), nil, nil, aws.String("NEEDLE"))
	require.NoError(t, err)
	assert.Len(t, all.List, len(fields))
	assert.Equal(t, int64(len(fields)), all.TotalCount)
}
