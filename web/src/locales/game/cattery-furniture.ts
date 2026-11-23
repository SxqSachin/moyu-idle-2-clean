import { furnitureV2 } from './cattery-furniture-v2'
import { trainingFurniture } from './cattery-training'

export const furnitureZhCN = {
  pendingStatus: '等待生效', pendingSeconds: '{seconds}秒后生效', pendingChange: '变更已提交，{seconds} 秒后切换至 Lv.{level} 的新状态。', changeDelay: '变更约 {seconds} 秒后生效；生效前保留原效果与维护计时。已开始的动作不受影响。', pendingError: '这件家具已有待生效变更，请生效后再操作。',
  afterInstall: '安置后效果', noMaintenance: '无需维护', shortDuration: '{hours}时 {minutes}分', backToDetails: '返回详情',
  ...furnitureV2.zhCN,
  ...trainingFurniture.zhCN,
  domains: { life_action: '采集与生产', fishing_cast: '钓鱼', battle_reward: '战斗胜利' },
  versionNotice: '当前为旧版耗材，不能混合补充。可用尽后补充，或升级时补齐差额。',
  sharedExperience: '分享给主人（已计入总经验）',
  studyDesk: '猫咪书桌', lootCabinet: '寻宝柜', install: '安置', refill: '补充耗材', enable: '启用', refresh: '刷新', checkResult: '确认上次操作结果',
  active: '使用中', paused: '已暂停', empty: '耗材用尽', notInstalled: '未安置', count: '补充份数',
  duration: '剩余 {hours} 小时 {minutes} 分钟', until: '预计用尽：{time}', addTime: '增加 {hours} 小时 {minutes} 分钟使用时间',
  workEffect: '猫咪干活时，额外将对应经验的 {rate}% 分享给主人。',
  lootEffect: '每只参战猫咪每场胜利有 {rate}% 概率参与额外寻宝，参与后仍可能空手而归。',
  workScope: '不扣减猫咪原有收益，不含强化、炼金、建造与战斗经验。',
  lootScope: '仅普通战斗掉落；不分走队友奖励，不含首通、迷雾与试炼奖励。',
  upkeep: '启用后按实际时间消耗耗材，闲置和离线期间也会消耗。暂停保留剩余时间，耗尽后只停止家具效果。',
  installNotice: '安置包含满额耗材，初始为暂停状态。确认后扣除以下库存材料。',
  refillNotice: '从库存扣除以下材料。不会自动续费，也不会让过去已开始的动作补享效果。',
  lootLog: '{name}：{result}', triggered: '参与额外寻宝', missed: '本次未参与寻宝',
  errors: {
    FURNITURE_UNAVAILABLE: '家具状态暂时无法读取，请刷新后重试。', FURNITURE_UNKNOWN: '操作结果尚未确认，请确认上次结果，不要重复补充。',
    FURNITURE_CONFLICT: '状态或配置已变化，已刷新，请重新确认。', FURNITURE_MATERIALS: '库存材料不足。',
    FURNITURE_RECOVERY_PENDING: '离线收益仍在结算，完成后才能扣除材料。现在仍可暂停家具。',
    FURNITURE_CAPACITY: '补充量超出容量，请减少份数。', FURNITURE_CATTERY_REQUIRED: '建造猫舍后可以安置家具。',
    FURNITURE_INVALID: '当前无法执行此操作，请刷新状态。', FURNITURE_HISTORY: '历史动作仍在结算，完成后可继续启用；现在仍可暂停。'
  }
}
export const furnitureEnUS = {
  pendingStatus: 'Scheduled', pendingSeconds: 'In {seconds}s', pendingChange: 'Change submitted. The new Lv.{level} state starts in {seconds}s.', changeDelay: 'Changes take effect in about {seconds} seconds. Until then, current effects and upkeep continue. Started operations stay unchanged.', pendingError: 'This furniture has a scheduled change. Wait for it to take effect.',
  afterInstall: 'Once installed', noMaintenance: 'No upkeep', shortDuration: '{hours}h {minutes}m', backToDetails: 'Back to details',
  ...furnitureV2.enUS,
  ...trainingFurniture.enUS,
  domains: { life_action: 'Gathering and production', fishing_cast: 'Fishing', battle_reward: 'Battle victories' },
  versionNotice: 'Legacy supplies cannot be mixed with refills. Use them up or pay the difference when upgrading.',
  sharedExperience: 'Shared with you (included in total XP)',
  studyDesk: 'Cat Study Desk', lootCabinet: 'Treasure Cabinet', install: 'Install', refill: 'Add supplies', enable: 'Enable', refresh: 'Refresh', checkResult: 'Check previous result',
  active: 'Active', paused: 'Paused', empty: 'Out of supplies', notInstalled: 'Not installed', count: 'Supply packs',
  duration: '{hours}h {minutes}m remaining', until: 'Runs out: {time}', addTime: 'Adds {hours}h {minutes}m of service',
  workEffect: 'Working cats share an additional {rate}% of their corresponding experience with you.',
  lootEffect: 'Each battle cat has a {rate}% chance per victory to seek extra loot.',
  workScope: 'Cat rewards stay unchanged. Excludes enhancement, alchemy, building and combat XP.',
  lootScope: 'Ordinary enemy drops only. No teammate, first-clear, fog or trial rewards.',
  upkeep: 'Supplies drain in real time while enabled, including idle and offline time. Pausing preserves the balance. Running out only disables furniture effects.',
  installNotice: 'Installation includes full supplies and starts paused. Confirm to spend these inventory materials.',
  refillNotice: 'Spend these inventory materials. No automatic refill or retroactive effects on operations already started.',
  lootLog: '{name}: {result}', triggered: 'Joined the extra loot search', missed: 'Did not join this search',
  errors: {
    FURNITURE_UNAVAILABLE: 'Furniture status is unavailable. Refresh to retry.', FURNITURE_UNKNOWN: 'The result is not confirmed. Check the previous result before spending again.',
    FURNITURE_CONFLICT: 'State or rules changed. Review the refreshed information.', FURNITURE_MATERIALS: 'Not enough materials.',
    FURNITURE_RECOVERY_PENDING: 'Offline rewards are still being settled. Wait before spending materials. You can still pause furniture.',
    FURNITURE_CAPACITY: 'This refill exceeds capacity. Reduce the pack count.', FURNITURE_CATTERY_REQUIRED: 'Build a cattery to install furniture.',
    FURNITURE_INVALID: 'This operation is unavailable. Refresh the state.', FURNITURE_HISTORY: 'Historical actions are still pending. Resume after recovery; pausing remains available.'
  }
}
export const furnitureZhTW = {
  pendingStatus: '等待生效', pendingSeconds: '{seconds}秒後生效', pendingChange: '變更已提交，{seconds} 秒後切換至 Lv.{level} 的新狀態。', changeDelay: '變更約 {seconds} 秒後生效；生效前保留原效果與維護計時。已開始的動作不受影響。', pendingError: '這件家具已有待生效變更，請生效後再操作。',
  afterInstall: '安置後效果', noMaintenance: '無需維護', shortDuration: '{hours}時 {minutes}分', backToDetails: '返回詳情',
  ...furnitureV2.zhTW,
  ...trainingFurniture.zhTW,
  domains: { life_action: '採集與生產', fishing_cast: '釣魚', battle_reward: '戰鬥勝利' },
  versionNotice: '目前為舊版耗材，不能混合補充。可用盡後補充，或升級時補齊差額。',
  sharedExperience: '分享給主人（已計入總經驗）',
  studyDesk: '貓咪書桌', lootCabinet: '尋寶櫃', install: '安置', refill: '補充耗材', enable: '啟用', refresh: '重新整理', checkResult: '確認上次操作結果',
  active: '使用中', paused: '已暫停', empty: '耗材用盡', notInstalled: '未安置', count: '補充份數',
  duration: '剩餘 {hours} 小時 {minutes} 分鐘', until: '預計用盡：{time}', addTime: '增加 {hours} 小時 {minutes} 分鐘使用時間',
  workEffect: '貓咪工作時，額外將對應經驗的 {rate}% 分享給主人。', lootEffect: '每隻參戰貓咪每場勝利有 {rate}% 機率參與額外尋寶，參與後仍可能空手而歸。',
  workScope: '不扣減貓咪原有收益，不含強化、煉金、建造與戰鬥經驗。', lootScope: '僅普通戰鬥掉落；不分走隊友獎勵，不含首通、迷霧與試煉獎勵。',
  upkeep: '啟用後按實際時間消耗耗材，閒置和離線期間也會消耗。暫停保留剩餘時間，耗盡後只停止家具效果。',
  installNotice: '安置包含滿額耗材，初始為暫停狀態。確認後扣除以下庫存材料。', refillNotice: '從庫存扣除以下材料。不會自動續費，也不會讓過去已開始的動作補享效果。',
  lootLog: '{name}：{result}', triggered: '參與額外尋寶', missed: '本次未參與尋寶',
  errors: {
    FURNITURE_UNAVAILABLE: '暫時無法讀取家具狀態，請重新整理後重試。', FURNITURE_UNKNOWN: '操作結果尚未確認，請確認上次結果，不要重複補充。',
    FURNITURE_CONFLICT: '狀態或設定已變更，請重新確認。', FURNITURE_MATERIALS: '庫存材料不足。', FURNITURE_RECOVERY_PENDING: '離線收益仍在結算，完成後才能扣除材料。目前仍可暫停家具。',
    FURNITURE_CAPACITY: '補充量超出容量，請減少份數。', FURNITURE_CATTERY_REQUIRED: '建造貓舍後可以安置家具。', FURNITURE_INVALID: '目前無法執行此操作，請重新整理。', FURNITURE_HISTORY: '歷史動作仍在結算，完成後可繼續啟用；目前仍可暫停。'
  }
}
export const furnitureJa = {
  pendingStatus: '適用待ち', pendingSeconds: '{seconds}秒後', pendingChange: '変更を受け付けました。{seconds}秒後にLv.{level}の新しい状態になります。', changeDelay: '変更は約{seconds}秒後に適用されます。それまでは現在の効果と維持費が続きます。開始済みの行動は変わりません。', pendingError: 'この家具には適用待ちの変更があります。適用後に再操作してください。',
  afterInstall: '設置後の効果', noMaintenance: '維持費なし', shortDuration: '{hours}時間 {minutes}分', backToDetails: '詳細に戻る',
  ...furnitureV2.ja,
  ...trainingFurniture.ja,
  domains: { life_action: '採集・生産', fishing_cast: '釣り', battle_reward: '戦闘勝利' },
  versionNotice: '旧資材への追加補充はできません。使い切るか、強化時に差額を支払います。',
  sharedExperience: '主人に共有（合計経験値に含まれます）',
  studyDesk: '猫の書斎机', lootCabinet: '宝探し棚', install: '設置', refill: '資材を補充', enable: '有効にする', refresh: '更新', checkResult: '前回の結果を確認',
  active: '稼働中', paused: '一時停止中', empty: '資材切れ', notInstalled: '未設置', count: '補充数', duration: '残り {hours} 時間 {minutes} 分', until: '資材切れ予定：{time}', addTime: '稼働時間を {hours} 時間 {minutes} 分追加',
  workEffect: '働く猫が対応する経験値の {rate}% を追加で主人に共有します。', lootEffect: '参戦した猫ごとに、勝利時 {rate}% の確率で追加の戦利品を探します。',
  workScope: '猫の報酬は減りません。強化・錬金・建築・戦闘経験値は対象外です。', lootScope: '通常の敵ドロップのみ。仲間の報酬、初回クリア、霧、試練の報酬は含みません。',
  upkeep: '有効中は待機中やオフライン中も実時間で資材を消費します。一時停止で残り時間を保存します。資材切れで止まるのは家具効果のみです。',
  installNotice: '資材満タン・一時停止状態で設置します。確定すると以下の所持資材を消費します。', refillNotice: '以下の所持資材を消費します。自動補充や開始済みの行動への遡及適用はありません。',
  lootLog: '{name}：{result}', triggered: '追加の宝探しに参加', missed: '今回は宝探しに不参加',
  errors: {
    FURNITURE_UNAVAILABLE: '家具の状態を取得できません。更新して再試行してください。', FURNITURE_UNKNOWN: '結果が未確認です。再補充の前に前回の結果を確認してください。',
    FURNITURE_CONFLICT: '状態または設定が変わりました。最新情報を確認してください。', FURNITURE_MATERIALS: '資材が足りません。', FURNITURE_RECOVERY_PENDING: 'オフライン報酬の精算中です。資材の消費は完了まで待ってください。一時停止は可能です。',
    FURNITURE_CAPACITY: '容量を超えています。補充数を減らしてください。', FURNITURE_CATTERY_REQUIRED: '猫舎を建てると家具を設置できます。', FURNITURE_INVALID: '現在この操作はできません。状態を更新してください。', FURNITURE_HISTORY: '過去の行動を精算中です。再開は精算後に行ってください。一時停止は可能です。'
  }
}
export const furnitureKo = {
  pendingStatus: '적용 대기', pendingSeconds: '{seconds}초 후 적용', pendingChange: '변경이 접수되었습니다. {seconds}초 후 Lv.{level}의 새 상태가 적용됩니다.', changeDelay: '변경은 약 {seconds}초 후 적용됩니다. 그때까지 기존 효과와 유지 비용이 계속됩니다. 시작한 행동은 바뀌지 않습니다.', pendingError: '적용 대기 중인 변경이 있습니다. 적용 후 다시 시도하세요.',
  afterInstall: '설치 후 효과', noMaintenance: '유지비 없음', shortDuration: '{hours}시간 {minutes}분', backToDetails: '상세로 돌아가기',
  ...furnitureV2.ko,
  ...trainingFurniture.ko,
  domains: { life_action: '채집 및 생산', fishing_cast: '낚시', battle_reward: '전투 승리' },
  versionNotice: '이전 소모품에 추가 보충할 수 없습니다. 소진 후 보충하거나 업그레이드 시 차액을 지불하세요.',
  sharedExperience: '주인에게 공유 (총 경험치에 포함)',
  studyDesk: '고양이 책상', lootCabinet: '보물찾기 수납장', install: '설치', refill: '소모품 보충', enable: '활성화', refresh: '새로고침', checkResult: '이전 작업 결과 확인',
  active: '사용 중', paused: '일시 정지', empty: '소모품 소진', notInstalled: '미설치', count: '보충 묶음 수', duration: '{hours}시간 {minutes}분 남음', until: '소진 예정: {time}', addTime: '사용 시간 {hours}시간 {minutes}분 추가',
  workEffect: '일하는 고양이가 해당 경험치의 {rate}%를 주인에게 추가로 공유합니다.', lootEffect: '참전한 고양이마다 승리 시 {rate}% 확률로 추가 전리품을 찾습니다.',
  workScope: '고양이 보상은 줄지 않습니다. 강화, 연금, 건설, 전투 경험치는 제외됩니다.', lootScope: '일반 적 드롭만 적용됩니다. 동료 보상, 최초 클리어, 안개, 시련 보상은 제외됩니다.',
  upkeep: '활성화 중에는 대기 및 오프라인 시간에도 실제 시간에 따라 소모품을 사용합니다. 일시 정지하면 남은 시간이 보존됩니다. 소진 시 가구 효과만 중단됩니다.',
  installNotice: '소모품이 가득 찬 일시 정지 상태로 설치됩니다. 확인 시 다음 인벤토리 재료를 소비합니다.', refillNotice: '다음 인벤토리 재료를 소비합니다. 자동 보충이나 이미 시작한 행동에 대한 소급 적용은 없습니다.',
  lootLog: '{name}: {result}', triggered: '추가 보물찾기에 참여', missed: '이번 보물찾기에 불참',
  errors: {
    FURNITURE_UNAVAILABLE: '가구 상태를 불러올 수 없습니다. 새로고침 후 다시 시도하세요.', FURNITURE_UNKNOWN: '결과가 확인되지 않았습니다. 다시 보충하기 전에 이전 결과를 확인하세요.',
    FURNITURE_CONFLICT: '상태 또는 설정이 바뀌었습니다. 최신 정보를 확인하세요.', FURNITURE_MATERIALS: '재료가 부족합니다.', FURNITURE_RECOVERY_PENDING: '오프라인 보상을 정산 중입니다. 재료 사용은 완료 후 가능합니다. 일시 정지는 가능합니다.',
    FURNITURE_CAPACITY: '최대 용량을 초과합니다. 보충 수를 줄이세요.', FURNITURE_CATTERY_REQUIRED: '고양이집을 지으면 가구를 설치할 수 있습니다.', FURNITURE_INVALID: '현재 이 작업을 할 수 없습니다. 상태를 새로고침하세요.', FURNITURE_HISTORY: '이전 행동을 정산 중입니다. 정산 후 재개할 수 있으며, 일시 정지는 가능합니다.'
  }
}
export const furnitureRu = {
  pendingStatus: 'Ожидает применения', pendingSeconds: 'Через {seconds}с', pendingChange: 'Изменение принято. Новое состояние ур.{level} вступит в силу через {seconds}с.', changeDelay: 'Изменения действуют примерно через {seconds} секунд. До этого сохраняются прежние эффекты и обслуживание. Начатые действия не меняются.', pendingError: 'Изменение уже ожидает применения. Дождитесь его завершения.',
  afterInstall: 'После установки', noMaintenance: 'Без обслуживания', shortDuration: '{hours}ч {minutes}м', backToDetails: 'К подробностям',
  ...furnitureV2.ru,
  ...trainingFurniture.ru,
  domains: { life_action: 'Сбор и производство', fishing_cast: 'Рыбалка', battle_reward: 'Победы в бою' },
  versionNotice: 'Старые запасы нельзя смешивать с новыми. Израсходуйте их или оплатите разницу при улучшении.',
  sharedExperience: 'Передано хозяину (включено в общий опыт)',
  studyDesk: 'Кошачий письменный стол', lootCabinet: 'Шкаф искателя сокровищ', install: 'Установить', refill: 'Пополнить запасы', enable: 'Включить', refresh: 'Обновить', checkResult: 'Проверить прошлую операцию',
  active: 'Работает', paused: 'На паузе', empty: 'Запасы закончились', notInstalled: 'Не установлено', count: 'Наборы припасов', duration: 'Осталось {hours} ч {minutes} мин', until: 'Закончатся: {time}', addTime: 'Добавляет {hours} ч {minutes} мин работы',
  workEffect: 'Работающие кошки дополнительно передают хозяину {rate}% соответствующего опыта.', lootEffect: 'Каждая участвующая в бою кошка с вероятностью {rate}% после победы ищет дополнительную добычу.',
  workScope: 'Награды кошки не уменьшаются. Не действует на усиление, алхимию, строительство и боевой опыт.', lootScope: 'Только обычная добыча врагов. Не затрагивает награды союзников, первого прохождения, тумана и испытаний.',
  upkeep: 'Во включённом состоянии запасы расходуются по реальному времени, даже при простое и офлайн. Пауза сохраняет остаток. При исчерпании запасов прекращается только эффект мебели.',
  installNotice: 'Установка включает полный запас и начинается на паузе. Подтверждение расходует указанные материалы из инвентаря.', refillNotice: 'Будут потрачены указанные материалы. Автопополнения и действия задним числом на уже начатые операции нет.',
  lootLog: '{name}: {result}', triggered: 'Участвует в поиске дополнительной добычи', missed: 'Не участвует в этом поиске',
  errors: {
    FURNITURE_UNAVAILABLE: 'Не удалось загрузить состояние мебели. Обновите и повторите.', FURNITURE_UNKNOWN: 'Результат ещё не подтверждён. Проверьте прошлую операцию перед новым пополнением.',
    FURNITURE_CONFLICT: 'Состояние или правила изменились. Проверьте свежие данные.', FURNITURE_MATERIALS: 'Недостаточно материалов.', FURNITURE_RECOVERY_PENDING: 'Идёт расчёт офлайн-наград. Дождитесь завершения перед тратой материалов. Пауза по-прежнему доступна.',
    FURNITURE_CAPACITY: 'Превышена вместимость. Уменьшите число наборов.', FURNITURE_CATTERY_REQUIRED: 'Постройте кошачий дом, чтобы установить мебель.', FURNITURE_INVALID: 'Сейчас операция недоступна. Обновите состояние.', FURNITURE_HISTORY: 'Предыдущие действия ещё рассчитываются. Включение доступно после расчёта, пауза доступна сейчас.'
  }
}
