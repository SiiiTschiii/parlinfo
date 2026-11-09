package main

import (
	"fmt"

	"github.com/siiitschiii/parlinfo/pkg/config"
	"github.com/siiitschiii/parlinfo/pkg/contacts"
	"github.com/siiitschiii/parlinfo/pkg/votes"
)

// sittingNames returns who sits in a jurisdiction. A jurisdiction whose source
// cannot say is an error rather than an empty list: read as empty chamber, it
// would make everyone who moved there look like a leaver.
func sittingNames(j config.Jurisdiction) ([]string, error) {
	if j.NewMemberSource == nil {
		return nil, fmt.Errorf("%s publishes no roster", j.Key)
	}
	lister, ok := j.NewMemberSource().(votes.SittingLister)
	if !ok {
		return nil, fmt.Errorf("%s cannot say who sits", j.Key)
	}
	return lister.SittingNames()
}

// sitsElsewhere returns the key of another jurisdiction each person sits in.
func sitsElsewhere(own string) (map[string]string, error) {
	elsewhere := map[string]string{}
	for _, key := range config.ChamberKeys() {
		if key == own {
			continue
		}
		other, err := config.LookupJurisdiction(key)
		if err != nil {
			return nil, err
		}
		names, err := sittingNames(other)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch who sits in %s: %w", key, err)
		}
		for _, name := range names {
			elsewhere[contacts.NameKey(name)] = key
		}
	}
	return elsewhere, nil
}

// partition splits the departed into those who sit nowhere and those who now sit
// in another chamber, keyed to which.
func partition(gone []string, elsewhere map[string]string) (leavers []string, movers map[string]string) {
	movers = map[string]string{}
	for _, name := range gone {
		if key, ok := elsewhere[contacts.NameKey(name)]; ok {
			movers[name] = key
			continue
		}
		leavers = append(leavers, name)
	}
	return leavers, movers
}

// checkPruneLimit refuses a prune large enough to suggest a broken source.
func checkPruneLimit(n, limit int) error {
	if n > limit {
		return fmt.Errorf("refusing to remove %d people (limit %d): check the source before raising -max-prune", n, limit)
	}
	return nil
}

// prune removes the named people from the file. Everyone it is given must be on
// file.
func prune(existing map[string]*Contact, names []string) {
	for _, name := range names {
		delete(existing, contacts.NameKey(name))
	}
}

// runPrune removes the people on file who sit in no chamber at all.
//
// Someone who now sits in another chamber is left in place and reported
// instead: their accounts belong in that chamber's file, and a posting channel
// loads only its own jurisdiction's files, so deleting them here would lose the
// accounts. A human moves them.
//
// A source that returns far fewer people than are on file is more likely broken
// than right, so a prune of more than maxPrune people is refused rather than
// written.
func runPrune(j config.Jurisdiction, path, header string, existing map[string]*Contact, maxPrune int, dryRun bool) error {
	sitting, err := sittingNames(j)
	if err != nil {
		return fmt.Errorf("failed to fetch who sits: %w", err)
	}
	elsewhere, err := sitsElsewhere(j.Key)
	if err != nil {
		return err
	}

	cs, _, _ := merge(existing, nil)

	leavers, movers := partition(departed(cs, sitting), elsewhere)
	for name, key := range movers {
		fmt.Printf("➡️  %s now sits in %s: left on file, move their accounts by hand\n", name, key)
	}

	for _, name := range leavers {
		fmt.Printf("➖ %s\n", name)
	}
	fmt.Printf("\n%d removed, nobody sits under that name in any chamber\n", len(leavers))

	if err := checkPruneLimit(len(leavers), maxPrune); err != nil {
		return err
	}
	if dryRun || len(leavers) == 0 {
		if dryRun {
			fmt.Println("🔍 Dry run — nothing written.")
		}
		return nil
	}

	prune(existing, leavers)
	merged, _, _ := merge(existing, nil)
	return save(path, header, merged)
}
