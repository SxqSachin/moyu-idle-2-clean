export function validateEquipmentSkills(templates, skills) {
  const byID = new Map(skills.map(row => [row.id, row]))
  const kinds = new Map([["equipment_domination", 1], ["equipment_martial_combo", 2]])
  for (const template of templates) {
    const grant = template.grantedSkill
    if (!grant) continue
    const skill = byID.get(grant.skillId)
    if (!kinds.has(grant.skillId) || !skill?.equipmentOnly || !skill.uninterruptible || skill.maxLevel !== 1 || template.equipType !== "battle" || template.slot !== "accessory" || template.enhanceMaxLevel !== 25 || !template.accessoryGroup || template.equipTargets?.join() !== "player") throw new Error(`equipment grant invalid: ${template.id}`)
    if (grant.baseCapacity !== kinds.get(grant.skillId) || JSON.stringify(grant.capacityMilestones) !== "[10,20]" || skill.effects?.length) throw new Error(`equipment skill capacity/effects invalid: ${template.id}`)
    if (grant.skillId === "equipment_domination" ? skill.passive !== true || skill.cooldown !== 0 : skill.passive === true || skill.cooldown !== 6) throw new Error(`equipment skill cast policy invalid: ${template.id}`)
  }
}
