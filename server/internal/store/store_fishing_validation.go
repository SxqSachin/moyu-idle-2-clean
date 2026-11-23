package store

import "strings"

// FishingValidationError preserves the sentinel used by queue cleanup while
// exposing a stable, player-safe reason. Fields are fixed slot names, not input.
type FishingValidationError struct {
	Reason string
	Field  string
	cause  error
}

func (e *FishingValidationError) Unwrap() error { return e.cause }

func (e *FishingValidationError) Code() string {
	code := "FISHING_SETTINGS_" + e.Reason
	if e.Field != "" {
		code += "_" + strings.ToUpper(e.Field)
	}
	return code
}

func (e *FishingValidationError) Error() string {
	message := fishingValidationMessages[e.Reason]
	if message == "" {
		message = "Fishing settings are invalid. Refresh and reselect your tackle."
	}
	if e.Field != "" {
		message += " (" + e.Field + ")"
	}
	return message
}

var fishingValidationMessages = map[string]string{
	"PAYLOAD_INVALID":         "Fishing request could not be read. Refresh and try again.",
	"SPOT_REQUIRED":           "Choose a fishing spot before departure.",
	"LOADOUT_REQUIRED":        "Choose a tackle set before departure.",
	"DIVISION_INVALID":        "Choose the open or standard event division.",
	"DURATION_INVALID":        "Choose a duration from 10 minutes to 24 hours, or fish until supplies run out.",
	"STRATEGY_INVALID":        "Reselect a conservative, balanced, or aggressive strategy.",
	"SAFETY_INVALID":          "Safety load must be between 50 and 100 percent.",
	"LAYER_INVALID":           "Reselect a valid target water layer.",
	"PACE_INVALID":            "Reselect a valid fishing pace.",
	"SUPPLY_RANGE":            "Supply quantity must be a whole number from 0 to 100000.",
	"SUPPLY_REQUIRED":         "Bring at least one of this supply.",
	"SUPPLY_UNEXPECTED":       "This supply is not used by the selected method. Set its quantity to zero.",
	"METHOD_INVALID":          "Edit the tackle set and select a valid fishing method.",
	"COMPONENT_REQUIRED":      "This tackle slot is empty. Edit the tackle set before departure.",
	"COMPONENT_INCOMPATIBLE":  "This tackle component does not match its slot or fishing method. Reselect it.",
	"EQUIPMENT_INVALID":       "This fishing equipment has an invalid configuration. Reselect it; report the error code if it persists.",
	"LOADOUT_CORRUPT":         "The saved tackle set could not be read. Save it again or create a new set.",
	"LOAD_CAPACITY_INVALID":   "The tackle set has no valid line load capacity. Reselect the main line and leader.",
	"SPOT_METHOD_MISMATCH":    "This spot does not support the tackle set's fishing method. Choose another spot or tackle set.",
	"BOTTOM_LAYER_MISMATCH":   "Bottom fishing requires the auto or near-bottom water layer.",
	"STANDARD_UNAVAILABLE":    "This event does not support the selected standard division. Refresh or choose the open division.",
	"STANDARD_MISMATCH":       "This tackle set does not match the event's specified tackle. Change the tackle or choose the open division.",
	"STANDARD_CONFIG_INVALID": "The event's specified tackle configuration is unavailable. Choose the open division and report the error code.",
	"RUNTIME_INVALID":         "The queued fishing plan is invalid. Cancel it and plan a new trip.",
	"LOADOUT_NAME_INVALID":    "Use a tackle name of at most 40 characters; recreate the set if the error persists.",
	"LOADOUT_LIMIT":           "The maximum number of tackle sets has been reached. Edit or delete an existing set.",
}

func fishingArgsError(reason, field string) error {
	return &FishingValidationError{Reason: reason, Field: field, cause: ErrFishingArgsInvalid}
}

func fishingLoadoutError(reason, field string) error {
	return &FishingValidationError{Reason: reason, Field: field, cause: ErrFishingLoadoutInvalid}
}
