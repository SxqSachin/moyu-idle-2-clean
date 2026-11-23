package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"

	"mysecondwebgame/server/internal/catalog"
)

func specializationTreeFromRaw(raw any) (specializationTreeSpec, error) {
	payload, err := json.Marshal(raw)
	if err != nil {
		return specializationTreeSpec{}, fmt.Errorf("%w: marshal tree: %v", ErrSpecializationDefinitionInvalid, err)
	}
	return specializationTreeFromJSON(payload)
}

func loadSpecializationCacheTestData(t *testing.T) catalog.Data {
	t.Helper()
	data, err := catalog.Load("../../../packages/game-data/source/json/specialization_trees.json")
	if err != nil {
		t.Fatalf("load specialization cache test data: %v", err)
	}
	if len(data.SpecializationTrees) == 0 {
		t.Fatal("specialization cache test data has no trees")
	}
	return data
}

func cloneSpecializationCacheTestTree(t *testing.T, tree catalog.SpecializationTree) catalog.SpecializationTree {
	t.Helper()
	payload, err := json.Marshal(tree)
	if err != nil {
		t.Fatalf("marshal specialization cache test tree: %v", err)
	}
	var out catalog.SpecializationTree
	if err := json.Unmarshal(payload, &out); err != nil {
		t.Fatalf("clone specialization cache test tree: %v", err)
	}
	return out
}

func specializationCacheTestCatalogPayload(tree *catalog.SpecializationTree) map[string]any {
	if tree == nil {
		return nil
	}
	for nodeIndex := range tree.Nodes {
		for levelIndex := range tree.Nodes[nodeIndex].Levels {
			for grantIndex := range tree.Nodes[nodeIndex].Levels[levelIndex].Grants {
				grant := &tree.Nodes[nodeIndex].Levels[levelIndex].Grants[grantIndex]
				if len(grant.BattleHooks) > 0 {
					return grant.BattleHooks[0]
				}
				if len(grant.EffectMutations) > 0 {
					return grant.EffectMutations[0]
				}
			}
		}
	}
	return nil
}

func specializationCacheTestGrant(tree specializationTreeSpec, kind string) (specializationNodeSpec, int, bool) {
	for _, node := range tree.Nodes {
		for levelIndex, level := range node.Levels {
			for _, grant := range level.Grants {
				if grant.Kind == kind {
					return node, levelIndex + 1, true
				}
			}
		}
	}
	return specializationNodeSpec{}, 0, false
}

func specializationCacheTestContributionPayload(contribution SpecializationBattleContribution, kind string) map[string]any {
	switch kind {
	case "battle_hook":
		if len(contribution.Hooks) > 0 {
			return contribution.Hooks[0].Payload
		}
	case "effect_mutation":
		if len(contribution.Mutations) > 0 {
			return contribution.Mutations[0].Payload
		}
	}
	return nil
}

func mutateSpecializationCacheTestNestedValue(raw any) bool {
	switch value := raw.(type) {
	case map[string]any:
		value["cacheNestedMutationProbe"] = true
		return true
	case []any:
		for _, item := range value {
			if mutateSpecializationCacheTestNestedValue(item) {
				return true
			}
		}
	case []map[string]any:
		for _, item := range value {
			if mutateSpecializationCacheTestNestedValue(item) {
				return true
			}
		}
	}
	return false
}

func mutateSpecializationCacheTestPayload(payload map[string]any) bool {
	if payload == nil {
		return false
	}
	payload["cacheMutationProbe"] = true
	for _, value := range payload {
		if mutateSpecializationCacheTestNestedValue(value) {
			return true
		}
	}
	return false
}

func TestSpecializationTreeCachePreservesOrderAndFirstEnabledLookup(t *testing.T) {
	base := loadSpecializationCacheTestData(t).SpecializationTrees[0]
	disabled := cloneSpecializationCacheTestTree(t, base)
	disabled.Disabled = true
	secondEnabled := cloneSpecializationCacheTestTree(t, base)
	secondEnabled.BalanceRevision++

	cache, err := buildSpecializationTreeCache([]catalog.SpecializationTree{disabled, base, secondEnabled})
	if err != nil {
		t.Fatal(err)
	}
	if len(cache.ordered) != 3 {
		t.Fatalf("ordered cache length=%d", len(cache.ordered))
	}
	index, ok := cache.firstEnabledByID[base.ID]
	if !ok || index != 1 {
		t.Fatalf("first enabled lookup index=%d exists=%t", index, ok)
	}
	if !cache.ordered[0].tree.Disabled || cache.ordered[1].tree.BalanceRevision != base.BalanceRevision || cache.ordered[2].tree.BalanceRevision != secondEnabled.BalanceRevision {
		t.Fatalf("cache order or definitions changed: %#v", cache.ordered)
	}
}

func TestSpecializationTreeCacheWarmLookupDoesNotAllocate(t *testing.T) {
	data := loadSpecializationCacheTestData(t)
	store := &Store{gameData: data}
	treeID := data.SpecializationTrees[0].ID
	if _, err := store.findSpecializationTree(treeID); err != nil {
		t.Fatal(err)
	}
	var lookupErr error
	allocations := testing.AllocsPerRun(1000, func() {
		_, lookupErr = store.findSpecializationTree(treeID)
	})
	if lookupErr != nil {
		t.Fatal(lookupErr)
	}
	if allocations != 0 {
		t.Fatalf("warm specialization cache lookup allocations=%f", allocations)
	}
}

func TestSpecializationTreeCacheRejectsInvalidTreeWithoutPartialPublish(t *testing.T) {
	base := loadSpecializationCacheTestData(t).SpecializationTrees[0]
	invalid := cloneSpecializationCacheTestTree(t, base)
	invalid.Disabled = true
	invalid.MaxActiveRootNodes++

	cache, err := buildSpecializationTreeCache([]catalog.SpecializationTree{base, invalid})
	if cache != nil || !errors.Is(err, ErrSpecializationDefinitionInvalid) {
		t.Fatalf("invalid trailing tree published a partial cache: cache=%#v err=%v", cache, err)
	}

	store := &Store{gameData: catalog.Data{SpecializationTrees: []catalog.SpecializationTree{base, invalid}}}
	if _, err := store.findSpecializationTree(base.ID); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
		t.Fatalf("lazy cache did not fail closed: %v", err)
	}
	store.gameData.SpecializationTrees = []catalog.SpecializationTree{base}
	if _, err := store.findSpecializationTree(base.ID); !errors.Is(err, ErrSpecializationDefinitionInvalid) {
		t.Fatalf("failed cache was rebuilt after publication attempt: %v", err)
	}
}

func TestStoreNewPanicsOnInvalidSpecializationCache(t *testing.T) {
	invalid := cloneSpecializationCacheTestTree(t, loadSpecializationCacheTestData(t).SpecializationTrees[0])
	invalid.MaxActiveRootNodes++
	defer func() {
		recovered := recover()
		err, ok := recovered.(error)
		if !ok || !errors.Is(err, ErrSpecializationDefinitionInvalid) {
			t.Fatalf("Store.New panic=%#v", recovered)
		}
	}()
	_ = New(nil, RuntimeConfig{}, catalog.Data{SpecializationTrees: []catalog.SpecializationTree{invalid}})
}

func TestSpecializationTreeCacheIsolatedFromCatalogAndRawViews(t *testing.T) {
	data := loadSpecializationCacheTestData(t)
	originalTreeJSON, err := json.Marshal(data.SpecializationTrees[0])
	if err != nil {
		t.Fatal(err)
	}
	store := &Store{gameData: data}
	entry, err := store.findSpecializationTree(data.SpecializationTrees[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	wantTreeJSON, err := json.Marshal(entry.tree)
	if err != nil {
		t.Fatal(err)
	}

	data.SpecializationTrees[0].RulesetVersion = "mutated-after-cache-build"
	if payload := specializationCacheTestCatalogPayload(&data.SpecializationTrees[0]); payload != nil {
		payload["cacheCatalogMutationProbe"] = true
	}
	entryAfterMutation, err := store.findSpecializationTree(entry.tree.ID)
	if err != nil {
		t.Fatal(err)
	}
	gotTreeJSON, err := json.Marshal(entryAfterMutation.tree)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotTreeJSON, wantTreeJSON) {
		t.Fatal("cached tree changed with the caller-owned catalog")
	}

	rawFirst, err := entry.rawTree()
	if err != nil {
		t.Fatal(err)
	}
	rawFirstJSON, err := json.Marshal(rawFirst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rawFirstJSON, originalTreeJSON) {
		t.Fatal("cached raw tree changed its API JSON contract")
	}
	rawFirst.RulesetVersion = "mutated-view"
	if payload := specializationCacheTestCatalogPayload(&rawFirst); payload != nil {
		payload["cacheRawMutationProbe"] = true
	}
	rawSecond, err := entry.rawTree()
	if err != nil {
		t.Fatal(err)
	}
	if rawSecond.RulesetVersion != entry.tree.RulesetVersion {
		t.Fatalf("cached raw tree was aliased: ruleset=%q", rawSecond.RulesetVersion)
	}
	if payload := specializationCacheTestCatalogPayload(&rawSecond); payload == nil || payload["cacheRawMutationProbe"] != nil {
		t.Fatalf("cached nested raw payload was aliased: %#v", payload)
	}
}

func TestSpecializationContributionDeepClonesRuntimePayloads(t *testing.T) {
	data := loadSpecializationCacheTestData(t)
	store := &Store{gameData: data}
	entry, err := store.findSpecializationTree(data.SpecializationTrees[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"battle_hook", "effect_mutation"} {
		t.Run(kind, func(t *testing.T) {
			node, rank, ok := specializationCacheTestGrant(entry.tree, kind)
			if !ok {
				t.Fatalf("real specialization tree has no %s grant", kind)
			}
			first := SpecializationBattleContribution{}
			if err := appendActiveSpecializationNodeContribution(&first, entry.tree, node, rank, "battle"); err != nil {
				t.Fatal(err)
			}
			firstPayload := specializationCacheTestContributionPayload(first, kind)
			baseline, err := json.Marshal(firstPayload)
			if err != nil {
				t.Fatal(err)
			}
			if !mutateSpecializationCacheTestPayload(firstPayload) {
				t.Fatalf("%s payload has no nested value to exercise deep cloning", kind)
			}

			second := SpecializationBattleContribution{}
			if err := appendActiveSpecializationNodeContribution(&second, entry.tree, node, rank, "battle"); err != nil {
				t.Fatal(err)
			}
			secondPayload := specializationCacheTestContributionPayload(second, kind)
			got, err := json.Marshal(secondPayload)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, baseline) {
				t.Fatalf("%s contribution mutation contaminated the cache\nwant=%s\ngot=%s", kind, baseline, got)
			}
		})
	}
}

func TestSpecializationTreeCacheConcurrentReadAndClone(t *testing.T) {
	data := loadSpecializationCacheTestData(t)
	store := &Store{gameData: data}
	const readers = 32
	errorsCh := make(chan error, readers)
	var wait sync.WaitGroup
	wait.Add(readers)
	for reader := 0; reader < readers; reader++ {
		go func(reader int) {
			defer wait.Done()
			entry, err := store.findSpecializationTree(data.SpecializationTrees[0].ID)
			if err != nil {
				errorsCh <- err
				return
			}
			raw, err := entry.rawTree()
			if err != nil {
				errorsCh <- err
				return
			}
			raw.RulesetVersion = fmt.Sprintf("reader-%d", reader)
			node, rank, ok := specializationCacheTestGrant(entry.tree, "battle_hook")
			if !ok {
				errorsCh <- errors.New("battle hook grant missing")
				return
			}
			contribution := SpecializationBattleContribution{}
			if err := appendActiveSpecializationNodeContribution(&contribution, entry.tree, node, rank, "battle"); err != nil {
				errorsCh <- err
				return
			}
			payload := specializationCacheTestContributionPayload(contribution, "battle_hook")
			payload["reader"] = reader
			if !mutateSpecializationCacheTestPayload(payload) {
				errorsCh <- errors.New("battle hook payload has no nested value")
			}
		}(reader)
	}
	wait.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatal(err)
		}
	}
}
