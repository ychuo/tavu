package views

import (
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"tavu/internal/files"
	"tavu/internal/gitrepo"
	"tavu/internal/review"
	sessions "tavu/internal/terminal"
)

// projectSessionState owns live in-memory workspace state, including the same
// editor buffers and PTY manager. It deliberately excludes App's atomics,
// window, global settings, cancellation functions and async request counters.
// Project state is parked only after its read jobs have been invalidated.
type projectSessionState struct {
	standalone bool
	gitWorkspaceState
	reviewDrafts            map[*document]reviewDraft
	statusEpoch             uint64
	latest                  map[string]string
	collapsedChanges        map[string]bool
	root                    string
	fs                      *files.Service
	repo                    *gitrepo.Repository
	status                  gitrepo.Status
	state                   projectData
	snapshot                *review.Snapshot
	reviewing               bool
	changed                 bool
	snapshotCheckNotice     string
	message                 string
	fileOpenError           string
	terms                   *sessions.Manager
	activeTerm              string
	secondTerm              string
	termFocus               string
	readerFocus             bool
	lastFocusedPane         string
	lastFocusedTerminal     string
	mode                    string
	readerRatio             float32
	doubleRatio             float32
	stackedRatio            float32
	arrangement             string
	sidebar                 bool
	sidebarWidth            float32
	sidebarTab              int
	reviewTab               int
	fileQuery               string
	textQuery               string
	revealPath              string
	searchFocus             bool
	searchCase              bool
	searchWord              bool
	searchRegex             bool
	searchDetails           bool
	searchOpenFiles         bool
	searchAllFiles          bool
	searchInclude           string
	searchExclude           string
	searchNotice            string
	searchCompleted         bool
	searchTruncated         bool
	searchSkipped           int
	searchCollapsed         map[string]bool
	searchSelected          string
	searchOpenPaths         string
	searchReplacementValues map[string]string
	searchReplacementNotice string
	searchDismissedFiles    map[string]bool
	searchDismissedMatches  []files.Match
	searchList              ui.ListState
	searchAnchor            *files.Match
	replaceUndo             *replacementUndo
	replaceFeedback         string
	replaceExpanded         bool
	replacement             string
	found                   []files.Entry
	matches                 []files.Match
	trees                   map[string][]files.Entry
	expanded                map[string]bool
	rows                    []treeRow
	listState               ui.ListState
	documents               []*document
	selectedDoc             string
	readerTabs              tabRevealState
	terminalTabs            tabRevealState
	diffLayout              int
	commentsOpen            bool
	commentsAll             bool
	commentsSelecting       bool
	commentsResolvedOpen    bool
	expandedComments        map[string]bool
	commentList             ui.ListState
	commentDraft            string
	editingComment          string
	commentEditDraft        string
	commentFile             bool
	commentFocus            bool
	selectedRange           editor.DiffLineRange
	hasRange                bool
	preview                 string
	exportSelected          map[string]bool
	commitMessage           string
	commitError             string
	commitErrorExpanded     bool
	commitDisplayedToken    string
}

func (a *App) captureProjectSession() *projectSessionState {
	return &projectSessionState{
		standalone:              a.standalone,
		gitWorkspaceState:       a.gitWorkspaceState,
		reviewDrafts:            a.reviewDrafts,
		statusEpoch:             a.statusEpoch,
		latest:                  a.latest,
		collapsedChanges:        a.collapsedChanges,
		root:                    a.root,
		fs:                      a.fs,
		repo:                    a.repo,
		status:                  a.status,
		state:                   a.state,
		snapshot:                a.snapshot,
		reviewing:               a.reviewing,
		changed:                 a.changed,
		snapshotCheckNotice:     a.snapshotCheckNotice,
		message:                 a.message,
		fileOpenError:           a.fileOpenError,
		terms:                   a.terms,
		activeTerm:              a.activeTerm,
		secondTerm:              a.secondTerm,
		termFocus:               a.termFocus,
		readerFocus:             a.readerFocus,
		lastFocusedPane:         a.lastFocusedPane,
		lastFocusedTerminal:     a.lastFocusedTerminal,
		mode:                    a.mode,
		readerRatio:             a.readerRatio,
		doubleRatio:             a.doubleRatio,
		stackedRatio:            a.stackedRatio,
		arrangement:             a.arrangement,
		sidebar:                 a.sidebar,
		sidebarWidth:            a.sidebarWidth,
		sidebarTab:              a.sidebarTab,
		reviewTab:               a.reviewTab,
		fileQuery:               a.fileQuery,
		textQuery:               a.textQuery,
		revealPath:              a.revealPath,
		searchFocus:             a.searchFocus,
		searchCase:              a.searchCase,
		searchWord:              a.searchWord,
		searchRegex:             a.searchRegex,
		searchDetails:           a.searchDetails,
		searchOpenFiles:         a.searchOpenFiles,
		searchAllFiles:          a.searchAllFiles,
		searchInclude:           a.searchInclude,
		searchExclude:           a.searchExclude,
		searchNotice:            a.searchNotice,
		searchCompleted:         a.searchCompleted,
		searchTruncated:         a.searchTruncated,
		searchSkipped:           a.searchSkipped,
		searchCollapsed:         a.searchCollapsed,
		searchSelected:          a.searchSelected,
		searchOpenPaths:         a.searchOpenPaths,
		searchReplacementValues: a.searchReplacementValues,
		searchReplacementNotice: a.searchReplacementNotice,
		searchDismissedFiles:    a.searchDismissedFiles,
		searchDismissedMatches:  a.searchDismissedMatches,
		searchList:              a.searchList,
		searchAnchor:            a.searchAnchor,
		replaceUndo:             a.replaceUndo,
		replaceFeedback:         a.replaceFeedback,
		replaceExpanded:         a.replaceExpanded,
		replacement:             a.replacement,
		found:                   a.found,
		matches:                 a.matches,
		trees:                   a.trees,
		expanded:                a.expanded,
		rows:                    a.rows,
		listState:               a.listState,
		documents:               a.documents,
		selectedDoc:             a.selectedDoc,
		readerTabs:              a.readerTabs,
		terminalTabs:            a.terminalTabs,
		diffLayout:              a.diffLayout,
		commentsOpen:            a.commentsOpen,
		commentsAll:             a.commentsAll,
		commentsSelecting:       a.commentsSelecting,
		commentsResolvedOpen:    a.commentsResolvedOpen,
		expandedComments:        a.expandedComments,
		commentList:             a.commentList,
		commentDraft:            a.commentDraft,
		editingComment:          a.editingComment,
		commentEditDraft:        a.commentEditDraft,
		commentFile:             a.commentFile,
		commentFocus:            a.commentFocus,
		selectedRange:           a.selectedRange,
		hasRange:                a.hasRange,
		preview:                 a.preview,
		exportSelected:          a.exportSelected,
		commitMessage:           a.commitMessage,
		commitError:             a.commitError,
		commitErrorExpanded:     a.commitErrorExpanded,
		commitDisplayedToken:    a.commitDisplayedToken,
	}
}

func (a *App) restoreProjectSession(s *projectSessionState) {
	guard := a.gitWriteGuard
	a.fileWorkspaceState = fileWorkspaceState{}
	a.reviewTransitionState = reviewTransitionState{reviewDrafts: s.reviewDrafts}
	a.gitWorkspaceState = s.gitWorkspaceState
	a.gitWriteGuard = guard
	a.statusEpoch = s.statusEpoch
	a.latest = s.latest
	a.collapsedChanges = s.collapsedChanges
	a.standalone = s.standalone
	a.root = s.root
	a.fs = s.fs
	a.repo = s.repo
	a.status = s.status
	a.state = s.state
	a.snapshot = s.snapshot
	a.reviewing = s.reviewing
	a.changed = s.changed
	a.snapshotCheckNotice = s.snapshotCheckNotice
	a.message = s.message
	a.fileOpenError = s.fileOpenError
	a.terms = s.terms
	a.activeTerm = s.activeTerm
	a.secondTerm = s.secondTerm
	a.termFocus = s.termFocus
	a.readerFocus = s.readerFocus
	a.lastFocusedPane = s.lastFocusedPane
	a.lastFocusedTerminal = s.lastFocusedTerminal
	a.mode = s.mode
	a.readerRatio = s.readerRatio
	a.doubleRatio = s.doubleRatio
	a.stackedRatio = s.stackedRatio
	a.arrangement = s.arrangement
	a.sidebar = s.sidebar
	a.sidebarWidth = s.sidebarWidth
	a.sidebarTab = s.sidebarTab
	a.reviewTab = s.reviewTab
	a.fileQuery = s.fileQuery
	a.textQuery = s.textQuery
	a.revealPath = s.revealPath
	a.searchFocus = s.searchFocus
	a.searchCase = s.searchCase
	a.searchWord = s.searchWord
	a.searchRegex = s.searchRegex
	a.searchDetails = s.searchDetails
	a.searchOpenFiles = s.searchOpenFiles
	a.searchAllFiles = s.searchAllFiles
	a.searchInclude = s.searchInclude
	a.searchExclude = s.searchExclude
	a.searchNotice = s.searchNotice
	a.searchCompleted = s.searchCompleted
	a.searchTruncated = s.searchTruncated
	a.searchSkipped = s.searchSkipped
	a.searchCollapsed = s.searchCollapsed
	a.searchSelected = s.searchSelected
	a.searchOpenPaths = s.searchOpenPaths
	a.searchReplacementValues = s.searchReplacementValues
	a.searchReplacementNotice = s.searchReplacementNotice
	a.searchDismissedFiles = s.searchDismissedFiles
	a.searchDismissedMatches = s.searchDismissedMatches
	a.searchList = s.searchList
	a.searchAnchor = s.searchAnchor
	a.replaceUndo = s.replaceUndo
	a.replaceFeedback = s.replaceFeedback
	a.replaceExpanded = s.replaceExpanded
	a.replacement = s.replacement
	a.found = s.found
	a.matches = s.matches
	a.trees = s.trees
	a.expanded = s.expanded
	a.rows = s.rows
	a.listState = s.listState
	a.documents = s.documents
	a.selectedDoc = s.selectedDoc
	a.readerTabs = s.readerTabs
	a.terminalTabs = s.terminalTabs
	a.diffLayout = s.diffLayout
	a.commentsOpen = s.commentsOpen
	a.commentsAll = s.commentsAll
	a.commentsSelecting = s.commentsSelecting
	a.commentsResolvedOpen = s.commentsResolvedOpen
	a.expandedComments = s.expandedComments
	a.commentList = s.commentList
	a.commentDraft = s.commentDraft
	a.editingComment = s.editingComment
	a.commentEditDraft = s.commentEditDraft
	a.commentFile = s.commentFile
	a.commentFocus = s.commentFocus
	a.selectedRange = s.selectedRange
	a.hasRange = s.hasRange
	a.preview = s.preview
	a.exportSelected = s.exportSelected
	a.commitMessage = s.commitMessage
	a.commitError = s.commitError
	a.commitErrorExpanded = s.commitErrorExpanded
	a.commitDisplayedToken = s.commitDisplayedToken
	a.treeLoading = map[string]bool{}
	a.settings.Layout.SplitTerminals = a.activeTerm != "" && a.secondTerm != ""
}
