package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/events"
	"github.com/steveyegge/gastown/internal/style"
	"github.com/steveyegge/gastown/internal/util"
)

// postDirectMergeNotify triggers the completion notification chain that the
// Refinery normally runs after merging an MR via the merge queue.
//
// When a polecat uses --merge=direct, it bypasses the Refinery entirely,
// pushing directly to main and force-closing the source issue. Without this
// function, the notification chain breaks:
//   - Mayor never receives MERGED (only SLOT_OPEN from witness later)
//   - Deacon is not notified to feed the next convoy issue
//   - Completed convoys are not auto-closed and subscribers are not notified
//   - Swarm integration branches are not landed
//
// This function mirrors the post-merge steps in Refinery's HandleMRInfoSuccess
// (engineer.go lines 1259-1273, postMergeConvoyCheck, notifyDeaconConvoyFeeding).
// All operations are best-effort: failures are logged but don't block gt done.
func postDirectMergeNotify(townRoot, rigName, issueID string, convoyInfo *ConvoyInfo) {
	// 1. Nudge mayor about direct merge (mirrors refinery HandleMRInfoSuccess line 1267-1273).
	// The witness sends SLOT_OPEN (polecat available) but mayor also needs MERGED
	// (work landed on main) to unblock dependent work and convoy tracking.
	nudgeMsg := fmt.Sprintf("MERGED: direct issue=%s", issueID)
	if convoyInfo != nil && convoyInfo.ID != "" {
		nudgeMsg = fmt.Sprintf("MERGED: direct issue=%s convoy=%s", issueID, convoyInfo.ID)
	}
	nudgeCmd := exec.Command("gt", "nudge", "mayor/", nudgeMsg)
	util.SetDetachedProcessGroup(nudgeCmd)
	nudgeCmd.Dir = townRoot
	if err := nudgeCmd.Run(); err != nil {
		style.PrintWarning("could not nudge mayor about direct merge: %v", err)
	} else {
		fmt.Printf("%s Mayor notified: MERGED (direct)\n", style.Bold.Render("✓"))
	}

	// 2. Convoy-related notifications (only if issue is part of a convoy)
	if convoyInfo == nil || convoyInfo.ID == "" {
		return
	}

	// 2a. Nudge deacon about convoy feeding (mirrors refinery notifyDeaconConvoyFeeding).
	// Without this, the deacon waits for the next patrol cycle (up to 10 minutes)
	// before discovering the issue closed and feeding the next ready issue.
	feedMsg := fmt.Sprintf("CONVOY_NEEDS_FEEDING: convoy=%s issue=%s", convoyInfo.ID, issueID)
	feedCmd := exec.Command("gt", "nudge", "deacon", feedMsg)
	util.SetDetachedProcessGroup(feedCmd)
	feedCmd.Dir = townRoot
	if err := feedCmd.Run(); err != nil {
		style.PrintWarning("could not nudge deacon about convoy feeding: %v", err)
	} else {
		fmt.Printf("%s Deacon notified: CONVOY_NEEDS_FEEDING\n", style.Bold.Render("✓"))
	}

	// 2b. Emit event to wake deacon from await-signal (mirrors refinery line 1926)
	_ = events.LogFeed(events.TypeMail, rigName+"/polecat", events.MailPayload("deacon/", "CONVOY_NEEDS_FEEDING "+convoyInfo.ID))

	// 2c. Check and close completed convoys + send completion notifications.
	// Reuses the existing checkAndCloseCompletedConvoys and notifyConvoyCompletion
	// functions from convoy.go (same logic the refinery uses via its own copy).
	townBeads := townRoot + "/.beads"
	if _, err := os.Stat(townBeads); err != nil {
		return // No town beads directory — nothing to check
	}
	closedConvoys, err := checkAndCloseCompletedConvoys(townBeads, false)
	if err != nil {
		style.PrintWarning("convoy completion check failed: %v", err)
	} else {
		for _, convoy := range closedConvoys {
			notifyConvoyCompletion(townBeads, convoy.ID, convoy.Title)
			landConvoySwarmAfterDirectMerge(townRoot, townBeads, convoy.ID)
		}
	}
}

// landConvoySwarmAfterDirectMerge lands a swarm integration branch for a
// completed convoy. This mirrors the refinery's landConvoySwarm logic.
func landConvoySwarmAfterDirectMerge(townRoot, townBeads, convoyID string) {
	out, err := runBdJSON(townBeads, "show", convoyID, "--json")
	if err != nil {
		return
	}
	var convoys []struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal(out, &convoys); err != nil || len(convoys) == 0 {
		return
	}

	fields := beads.ParseConvoyFields(&beads.Issue{Description: convoys[0].Description})
	if fields == nil || fields.Molecule == "" {
		return
	}

	moleculeID := fields.Molecule
	landCmd := exec.Command("gt", "swarm", "land", moleculeID)
	util.SetDetachedProcessGroup(landCmd)
	landCmd.Dir = townRoot
	if err := landCmd.Run(); err != nil {
		style.PrintWarning("convoy landing: failed to land swarm %s for convoy %s: %v", moleculeID, convoyID, err)
	} else {
		fmt.Printf("%s Landed integration branch for convoy %s\n", style.Bold.Render("✓"), convoyID)
	}
}