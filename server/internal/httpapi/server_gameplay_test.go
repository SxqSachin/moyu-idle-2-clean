package httpapi

import (
	"reflect"
	"testing"

	"mysecondwebgame/server/internal/catalog"
)

func TestAccessoryTemplateIDsByTemplateReturnsOnlyTemplateIDForAccessory(t *testing.T) {
	got := accessoryTemplateIDsByTemplate(&catalog.EquipmentTemplate{
		ID:   "ice_gou_charm",
		Slot: "accessory",
	})

	want := []string{"ice_gou_charm"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("accessoryTemplateIDsByTemplate() = %v, want %v", got, want)
	}
}

func TestAccessoryTemplateIDsByTemplateIgnoresNonAccessoryTemplates(t *testing.T) {
	got := accessoryTemplateIDsByTemplate(&catalog.EquipmentTemplate{
		ID:   "wood_sword",
		Slot: "right_hand",
	})

	if got != nil {
		t.Fatalf("accessoryTemplateIDsByTemplate() = %v, want nil", got)
	}
}
