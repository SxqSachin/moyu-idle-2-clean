/** How to find and prepare the target element in the DOM */
export type TargetDescriptor = {
  /** Route to navigate to before finding the element */
  route?: string
  /** CSS selector to locate the element */
  selector?: string
  /** Async setup: open dialog, switch tab, expand section, etc. Runs after navigation. */
  setup?: () => Promise<void> | void
  /** Max ms to wait for element to appear in DOM (default: 5000) */
  waitForMs?: number
  /** Continue even if selector is not found before timeout. Popover will be centered. */
  optional?: boolean
}

/** Popover display config — translated via i18n keys */
export type TutorialPopover = {
  titleKey: string
  descriptionKey: string
  side?: "top" | "right" | "bottom" | "left" | "over"
  align?: "start" | "center" | "end"
  popoverClass?: string
}

/** Mobile-specific overrides for a tutorial step */
export type TutorialStepMobileOverrides = {
  target?: Partial<TargetDescriptor>
  popover?: Partial<TutorialPopover>
  when?: () => boolean
}

/** A single step in a tutorial sequence */
export type TutorialStep = {
  id: string
  target: TargetDescriptor
  popover: TutorialPopover
  /** Skip this step if condition returns false */
  when?: () => boolean
  /** Move to the next tutorial step after the highlighted target is clicked. */
  advanceOnTargetClick?: boolean
  /**
   * Allow the user to actually interact with the highlighted element (click,
   * type). Overrides the global `disableActiveInteraction`. Use for hands-on
   * finale steps where the player performs the action themselves.
   */
  interactive?: boolean
  /** Move to the next tutorial step when this selector appears. Useful after user-triggered dialogs. */
  advanceWhen?: {
    selector: string
    waitForMs?: number
  }
  /** Mobile-specific overrides merged on top of base step when isMobile */
  mobile?: TutorialStepMobileOverrides
}

/** Tutorial category for grouping in the guide dialog */
export type TutorialCategory = "core" | "economy" | "progression" | "social"

/** A complete tutorial sequence for one module */
export type TutorialConfig = {
  id: string
  nameKey: string
  descriptionKey: string
  icon?: string
  steps: TutorialStep[]
  /** Auto-start on first visit to the module's route */
  autoStart?: boolean
  /** Increment to force re-completion for users who finished older version */
  version?: number
  /** Tutorial IDs that must be completed before this one unlocks */
  prerequisites?: string[]
  /** Category for grouping in the guide dialog */
  category?: TutorialCategory
  /** Display order within its category (lower = first) */
  order?: number
}

/** Persisted progress for one tutorial */
export type TutorialProgress = {
  completed: boolean
  completedAt: number | null
  version: number
}
