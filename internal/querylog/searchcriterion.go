package querylog

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/AdguardTeam/AdGuardHome/internal/filtering"
	"github.com/AdguardTeam/golibs/container"
	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/stringutil"
)

type criterionType int

const (
	// ctTerm is for searching by the domain name, the client's IP address,
	// the client's ID or the client's name.  The domain name search
	// supports IDNAs.
	ctTerm criterionType = iota

	// ctFilteringStatus is for searching by the filtering status.
	//
	// See (*searchCriterion).ctFilteringStatusCase for details.
	//
	// Deprecated: Remove when migration to reason criterion is complete.
	ctFilteringStatus

	// ctReason is for searching by the filtering reason.
	ctReason
)

const (
	filteringStatusAll      = "all"
	filteringStatusFiltered = "filtered" // all kinds of filtering

	filteringStatusBlocked             = "blocked"              // blocked or blocked services
	filteringStatusBlockedService      = "blocked_services"     // blocked
	filteringStatusBlockedSafebrowsing = "blocked_safebrowsing" // blocked by safebrowsing
	filteringStatusBlockedParental     = "blocked_parental"     // blocked by parental control
	filteringStatusBlockedGames        = "blocked_games"        // blocked by game control
	filteringStatusWhitelisted         = "whitelisted"          // whitelisted
	filteringStatusRewritten           = "rewritten"            // all kinds of rewrites
	filteringStatusSafeSearch          = "safe_search"          // enforced safe search
	filteringStatusProcessed           = "processed"            // not blocked, not white-listed entries
)

// filteringStatusValues is the set of all possible [filteringStatus] values.
var filteringStatusValues = container.NewMapSet(
	filteringStatusAll,
	filteringStatusBlocked,
	filteringStatusBlockedGames,
	filteringStatusBlockedParental,
	filteringStatusBlockedSafebrowsing,
	filteringStatusBlockedService,
	filteringStatusFiltered,
	filteringStatusProcessed,
	filteringStatusRewritten,
	filteringStatusSafeSearch,
	filteringStatusWhitelisted,
)

// reasonCodes is a set of all valid reason codes.
var reasonCodes = [...]string{
	filtering.NotFilteredAllowList:   "1",
	filtering.NotFilteredError:       "2",
	filtering.FilteredBlockList:      "3",
	filtering.FilteredSafeBrowsing:   "4",
	filtering.FilteredParental:       "5",
	filtering.FilteredInvalid:        "6",
	filtering.FilteredSafeSearch:     "7",
	filtering.FilteredBlockedService: "8",
	filtering.Rewritten:              "9",
	filtering.RewrittenAutoHosts:     "10",
	filtering.RewrittenRule:          "11",
}

// searchCriterion is a search criterion that is used to match a record.
type searchCriterion struct {
	// value is the target value for searching.  If
	// [searchCriterion.criterionType] is [ctTerm] or [ctFilteringStatus] value
	// must not be empty.
	value string

	// asciiVal is the ASCII representation of value for matching IDNA domain
	// names.  It is used by [ctTerm].
	asciiVal string

	// values is a list of target values for searching.  It is used by
	// [ctReason] type.
	values []string

	// criterionType is the type of search criterion.  It must be one of:
	//	- [ctTerm]
	//	- [ctFilteringStatus]
	//	- [ctReason]
	criterionType criterionType

	// strict, if true, means that the criterion must be applied to the whole
	// value rather than the part of it.  That is, equality and not containment.
	// It is used by [ctTerm].
	strict bool
}

func ctDomainOrClientCaseStrict(
	term string,
	asciiTerm string,
	clientID string,
	name string,
	host string,
	ip string,
) (ok bool) {
	return strings.EqualFold(host, term) ||
		(asciiTerm != "" && strings.EqualFold(host, asciiTerm)) ||
		strings.EqualFold(clientID, term) ||
		strings.EqualFold(ip, term) ||
		strings.EqualFold(name, term)
}

// gamesKeywords, parentalKeywords, and safebrowsingKeywords are the
// substrings used to recognize why a query was blocked, both from the raw
// query-log line and from the text of the specific filter rule that matched.
var (
	gamesKeywords        = []string{"games", "gamecontrol", "poki"}
	parentalKeywords     = []string{"nsfw", "porn", "adult", "xvideos", "oisd"}
	safebrowsingKeywords = []string{
		"abuse", "malware", "malicious", "threat", "badware", "phishing",
		"openphish", "urlhaus", "scam", "tif", "security", "rebind",
	}
)

// filteringStatusKeywords maps a keyword-based [filteringStatus] to the
// substrings that identify it in a query-log line or filter rule.
var filteringStatusKeywords = map[string][]string{
	filteringStatusBlockedGames:        gamesKeywords,
	filteringStatusBlockedParental:     parentalKeywords,
	filteringStatusBlockedSafebrowsing: safebrowsingKeywords,
}

// containsAny returns true if s contains any of the given substrings.
func containsAny(s string, substrs []string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}

	return false
}

func ctDomainOrClientCaseNonStrict(
	term string,
	asciiTerm string,
	clientID string,
	name string,
	host string,
	ip string,
) (ok bool) {
	return stringutil.ContainsFold(clientID, term) ||
		stringutil.ContainsFold(host, term) ||
		(asciiTerm != "" && stringutil.ContainsFold(host, asciiTerm)) ||
		stringutil.ContainsFold(ip, term) ||
		stringutil.ContainsFold(name, term)
}

// quickMatch quickly checks if the line matches the given search criterion.
// It returns false if the like doesn't match.  This method is only here for
// optimization purposes.  logger and findClient must not be nil.
func (c *searchCriterion) quickMatch(
	ctx context.Context,
	logger *slog.Logger,
	line string,
	findClient quickMatchClientFunc,
) (ok bool) {
	switch c.criterionType {
	case ctTerm:
		return c.quickMatchTerm(ctx, logger, line, findClient)
	case ctFilteringStatus:
		return quickMatchFilteringStatus(line, c.value)
	case ctReason:
		return c.quickMatchReason(line)
	default:
		return true
	}
}

// quickMatchTerm is the [ctTerm] case of quickMatch.  logger and findClient
// must not be nil.
func (c *searchCriterion) quickMatchTerm(
	ctx context.Context,
	logger *slog.Logger,
	line string,
	findClient quickMatchClientFunc,
) (ok bool) {
	host := readJSONValue(line, `"QH":"`)
	ip := readJSONValue(line, `"IP":"`)
	clientID := readJSONValue(line, `"CID":"`)

	var name string
	if cli := findClient(ctx, logger, clientID, ip); cli != nil {
		name = cli.Name
	}

	if c.strict {
		return ctDomainOrClientCaseStrict(c.value, c.asciiVal, clientID, name, host, ip)
	}

	return ctDomainOrClientCaseNonStrict(c.value, c.asciiVal, clientID, name, host, ip)
}

// quickMatchFilteringStatus is the [ctFilteringStatus] case of quickMatch.
func quickMatchFilteringStatus(line, value string) (ok bool) {
	if value == filteringStatusBlocked {
		return strings.Contains(line, `"IsFiltered":true`)
	}

	keywords, isKeywordStatus := filteringStatusKeywords[value]
	if !isKeywordStatus {
		return true
	}

	return strings.Contains(line, `"IsFiltered":true`) && containsAny(strings.ToLower(line), keywords)
}

// quickMatchReason is the [ctReason] case of quickMatch.
func (c *searchCriterion) quickMatchReason(line string) (ok bool) {
	reasonCode := readJSONNumericValue(line, `"Reason":`)
	if reasonCode == "" {
		// For [filtering.NotFilteredNotFound] reason can be empty.
		return slices.Contains(c.values, filtering.NotFilteredNotFound.String())
	}

	idx := slices.Index(reasonCodes[:], reasonCode)
	if idx == -1 {
		return false
	}

	return slices.Contains(c.values, filtering.Reason(idx).String())
}

// match checks if the log entry matches this search criterion.  entry must not
// be nil.
func (c *searchCriterion) match(entry *logEntry) bool {
	switch c.criterionType {
	case ctTerm:
		return c.ctDomainOrClientCase(entry)
	case ctFilteringStatus:
		return c.ctFilteringStatusCase(entry.Result.Reason, entry.Result.IsFiltered, entry.Result.Rules)
	case ctReason:
		// TODO(f.setrakov): Consider comparing [filtering.Reason] instead of
		// strings.
		return slices.Contains(c.values, entry.Result.Reason.String())
	}

	return false
}

func (c *searchCriterion) ctDomainOrClientCase(e *logEntry) bool {
	clientID := e.ClientID
	host := e.QHost

	var name string
	if e.client != nil {
		name = e.client.Name
	}

	ip := e.IP.String()
	if c.strict {
		return ctDomainOrClientCaseStrict(c.value, c.asciiVal, clientID, name, host, ip)
	}

	return ctDomainOrClientCaseNonStrict(c.value, c.asciiVal, clientID, name, host, ip)
}

// ctFilteringStatusCase returns true if the result matches the value.
func (c *searchCriterion) ctFilteringStatusCase(
	reason filtering.Reason,
	isFiltered bool,
	rules []*filtering.ResultRule,
) (matched bool) {
	switch c.value {
	case filteringStatusAll:
		return true
	case filteringStatusFiltered:
		return isFiltered || reason == filtering.NotFilteredAllowList || reasonIsRewrite(reason)
	case
		filteringStatusBlocked,
		filteringStatusBlockedGames,
		filteringStatusBlockedParental,
		filteringStatusBlockedSafebrowsing,
		filteringStatusBlockedService,
		filteringStatusSafeSearch:
		return isFiltered && c.isFilteredWithReason(reason, rules)
	case filteringStatusWhitelisted:
		return reason == filtering.NotFilteredAllowList
	case filteringStatusRewritten:
		return reasonIsRewrite(reason)
	case filteringStatusProcessed:
		return !reasonIsRuleList(reason)
	default:
		return false
	}
}

// reasonIsRewrite returns true if r is one of:
//
//   - [filtering.RewrittenAutoHosts]
//   - [filtering.RewrittenRule]
//   - [filtering.Rewritten]
func reasonIsRewrite(r filtering.Reason) (ok bool) {
	return r == filtering.RewrittenAutoHosts ||
		r == filtering.RewrittenRule ||
		r == filtering.Rewritten
}

// isFilteredWithReason returns true if reason matches the criterion value.
// c.value must be one of:
//
//   - [filteringStatusBlockedParental]
//   - [filteringStatusBlockedSafebrowsing]
//   - [filteringStatusBlockedService]
//   - [filteringStatusBlocked]
//   - [filteringStatusSafeSearch]
func (c *searchCriterion) isFilteredWithReason(reason filtering.Reason, rules []*filtering.ResultRule) (matched bool) {
	switch c.value {
	case filteringStatusBlocked:
		return reason == filtering.FilteredBlockList || reason == filtering.FilteredBlockedService
	case filteringStatusBlockedParental:
		return reason == filtering.FilteredParental || matchParentalRule(rules)
	case filteringStatusBlockedSafebrowsing:
		return reason == filtering.FilteredSafeBrowsing || matchSafebrowsingRule(rules)
	case filteringStatusBlockedGames:
		return matchGamesRule(rules)
	case filteringStatusBlockedService:
		return reason == filtering.FilteredBlockedService
	case filteringStatusSafeSearch:
		return reason == filtering.FilteredSafeSearch
	default:
		panic(fmt.Errorf("%w: %q", errors.ErrBadEnumValue, c.value))
	}
}

func matchParentalRule(rules []*filtering.ResultRule) bool {
	if len(rules) == 0 {
		return false
	}

	return containsAny(strings.ToLower(rules[0].Text), parentalKeywords)
}

func matchSafebrowsingRule(rules []*filtering.ResultRule) bool {
	if len(rules) == 0 {
		return false
	}

	return containsAny(strings.ToLower(rules[0].Text), safebrowsingKeywords)
}

// matchGamesRule reports whether the first rule's text marks the query as
// blocked by the games block list, GameControl, or "games" specifically.
func matchGamesRule(rules []*filtering.ResultRule) bool {
	if len(rules) == 0 {
		return false
	}

	return containsAny(strings.ToLower(rules[0].Text), gamesKeywords)
}

// reasonIsRuleList returns true if r is one of:
//
//   - [filtering.FilteredBlockList]
//   - [filtering.FilteredBlockedService]
//   - [filtering.NotFilteredAllowList]
func reasonIsRuleList(r filtering.Reason) (ok bool) {
	return r == filtering.FilteredBlockList ||
		r == filtering.FilteredBlockedService ||
		r == filtering.NotFilteredAllowList
}
