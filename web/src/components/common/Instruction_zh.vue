<template>
  <section class="min-h-screen bg-slate-50 px-4 py-8 text-slate-900 sm:px-6 lg:px-8">
    <div class="mx-auto max-w-6xl">
      <header class="overflow-hidden rounded-3xl border border-slate-200 bg-white shadow-sm">
        <div class="relative px-6 py-8 sm:px-8 lg:px-10">
          <div class="absolute right-0 top-0 h-40 w-40 rounded-full bg-amber-100 blur-3xl" />
          <div class="absolute bottom-0 right-24 h-32 w-32 rounded-full bg-sky-100 blur-3xl" />

          <div class="relative flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
            <div>
              <p class="mb-3 inline-flex rounded-full bg-slate-100 px-3 py-1 text-xs font-medium text-slate-600">
                测试阶段玩家守则与免责声明
              </p>
              <h1 class="text-3xl font-bold tracking-tight text-slate-950 sm:text-4xl">
                《摸鱼放置》玩家守则
              </h1>
              <p class="mt-4 max-w-3xl text-sm leading-7 text-slate-600 sm:text-base">
                本项目当前主要用于个人学习、技术验证和项目展示。当前版本为早期测试版本，并非正式发布版本。
                参与测试前，请仔细阅读以下规则、风险提示、账号处罚规则与免责声明。
              </p>
            </div>
<!-- 
            <div class="grid gap-3 rounded-2xl bg-slate-950 p-4 text-sm text-white shadow-sm sm:min-w-72">
              <div class="flex items-center justify-between gap-4">
                <span class="text-slate-300">版本号</span>
                <span class="font-semibold">{{ version }}</span>
              </div>
              <div class="flex items-center justify-between gap-4">
                <span class="text-slate-300">生效日期</span>
                <span class="font-semibold">{{ effectiveDate }}</span>
              </div>
              <div class="flex items-center justify-between gap-4">
                <span class="text-slate-300">项目方</span>
                <span class="font-semibold">{{ operatorName }}</span>
              </div>
              <div class="flex items-center justify-between gap-4">
                <span class="text-slate-300">联系邮箱</span>
                <a class="font-semibold text-sky-200 hover:text-sky-100" :href="`mailto:${contactEmail}`">
                  {{ contactEmail }}
                </a>
              </div>
            </div> -->
          </div>
        </div>
      </header>

      <div class="mt-6 grid gap-6 lg:grid-cols-[280px_1fr]">
        <aside class="lg:sticky lg:top-6 lg:self-start">
          <div class="rounded-3xl border border-slate-200 bg-white p-4 shadow-sm">
            <div class="mb-4 flex items-center justify-between">
              <h2 class="text-sm font-bold text-slate-950">目录</h2>
            </div>

            <nav class="space-y-1">
              <a
                v-for="section in sections"
                :key="section.id"
                :href="`#${section.id}`"
                class="block rounded-2xl px-3 py-2 text-sm text-slate-600 transition hover:bg-slate-100 hover:text-slate-950"
              >
                {{ section.title }}
              </a>
            </nav>
          </div>

          <div class="mt-4 rounded-3xl border border-amber-200 bg-amber-50 p-4 text-sm leading-6 text-amber-950 shadow-sm">
            <p class="font-bold">重点提示</p>
            <p class="mt-2">
              测试数据不具有现实货币价值，不承诺永久保留。违规交易、多开账号、外挂脚本、漏洞利用等行为可能导致账号永久封禁。
            </p>
          </div>
        </aside>

        <main class="space-y-5">
          <div class="grid gap-4 md:grid-cols-3">
            <InfoCard
              title="非正式版本"
              content="当前为早期测试体验，不代表正式发布、长期服务或商业化运营承诺。"
            />
            <InfoCard
              title="禁止交易"
              content="账号、道具、金币、资源、排行榜名次等不得进行现实货币交易或变相交易。"
            />
            <InfoCard
              title="违规可封禁"
              content="外挂、脚本、多开矩阵、资源输送、漏洞利用、RMT 等严重违规可永久封禁。"
            />
          </div>

          <article
            v-for="section in sections"
            :id="section.id"
            :key="section.id"
            class="scroll-mt-6 rounded-3xl border border-slate-200 bg-white shadow-sm"
          >
            <div
              class="flex w-full items-start justify-between gap-4 px-5 py-5 text-left sm:px-6"
            >
              <div>
                <p class="text-xs font-semibold uppercase tracking-wide text-slate-400">{{ section.badge }}</p>
                <h2 class="mt-1 text-xl font-bold text-slate-950">{{ section.title }}</h2>
                <p v-if="section.summary" class="mt-2 text-sm leading-6 text-slate-600">
                  {{ section.summary }}
                </p>
              </div>
            </div>

            <div class="border-t border-slate-100 px-5 pb-6 pt-2 sm:px-6">
              <ol class="space-y-3 text-sm leading-7 text-slate-700">
                <li
                  v-for="(item, index) in section.items"
                  :key="`${section.id}-${index}`"
                  class="flex gap-3"
                >
                  <span class="mt-1 flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-slate-100 text-xs font-bold text-slate-500">
                    {{ index + 1 }}
                  </span>
                  <span>{{ item }}</span>
                </li>
              </ol>
            </div>
          </article>
        </main>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { defineComponent, ref } from 'vue'

type PolicySection = {
  id: string
  badge: string
  title: string
  summary?: string
  items: string[]
}

const props = withDefaults(
  defineProps<{
    version?: string
    effectiveDate?: string
    operatorName?: string
    contactEmail?: string
  }>(),
  {
    version: 'V1.0',
    effectiveDate: '请填写日期',
    operatorName: '请填写项目方名称',
    contactEmail: 'example@example.com',
  },
)

const emit = defineEmits<{
  accept: []
}>()

const { version, effectiveDate, operatorName, contactEmail } = props
const accepted = ref(false)

const sections: PolicySection[] = [
  {
    id: 'project-nature',
    badge: '01',
    title: '项目性质说明',
    summary: '明确《摸鱼放置》当前为个人学习、技术验证和项目展示用途的早期测试版本。',
    items: [
      '《摸鱼放置》目前处于早期测试阶段，项目当前主要用于个人学习、技术验证、项目展示、玩法实验、服务器稳定性测试、数值平衡测试和用户体验收集。',
      '当前测试版本不代表正式发布版本，也不构成项目方对正式版本、长期服务、持续运营、商业化上线、数据保留、功能保留、版本更新或后续维护的承诺。',
      '当前测试阶段可能存在未完成内容、程序错误、数值异常、服务器不稳定、功能缺失、文案错误、界面错误、数据异常、兼容性问题及其他测试阶段常见问题。',
      '项目方有权基于学习、测试、展示、技术调整、成本控制、安全风控、合规要求或其他合理原因，随时调整、暂停、关闭、重置、回滚或终止全部或部分服务。',
      '测试阶段开放访问或允许注册，不代表《摸鱼放置》已进入正式发布、正式运营、商业化运营或长期稳定服务阶段。',
    ],
  },
  {
    id: 'account-rules',
    badge: '02',
    title: '注册、登录与账号规则',
    summary: '限制账号共享、交易、盗号、批量注册等行为，并保留项目方限制或封禁账号的权利。',
    items: [
      '玩家可通过项目方提供的官方入口注册账号并体验测试版本。玩家应保证注册、登录和使用行为真实、合法、合规。',
      '玩家应自行妥善保管账号、密码、验证码、邮箱、第三方登录凭证、设备、浏览器缓存及其他登录凭证。',
      '因玩家自行泄露账号信息、共享账号、使用弱密码、访问非官方链接、下载非官方客户端、相信诈骗信息等原因造成的损失，由玩家自行承担。',
      '玩家不得出售、出租、转让、共享、赠与、抵押账号，不得进行账号交易、账号代练、账号代刷、批量注册、恶意注册、脚本注册、撞库登录、盗号或冒用他人身份。',
      '项目方有权根据服务器容量、异常注册情况、账号安全、风控结果、测试需要、技术维护、违规行为或其他合理原因，限制、暂停、冻结或封禁账号。',
    ],
  },
  {
    id: 'non-commercial',
    badge: '03',
    title: '非商业化测试说明',
    summary: '明确当前测试阶段不提供游戏内充值、内购、广告激励或其他商业化收费服务。',
    items: [
      '当前测试阶段不在游戏内提供充值、内购、付费购买、付费抽取、付费加速、付费解锁、广告激励、虚拟货币购买、虚拟道具现金购买或其他游戏内商业化收费服务。',
      '游戏内金币、材料、装备、道具、宠物、技能、经验、等级、称号、排行榜奖励、活动奖励、测试奖励及其他虚拟内容，仅用于测试体验，不具有现实货币价值。',
      '任何游戏内虚拟物品、虚拟货币、账号数据、角色数据、排行榜数据、测试数据均不得兑换现金、实物、证券、积分、平台权益、第三方权益或任何现实收益。',
      '未经项目方明确书面说明，任何游戏外的自愿支持、打赏、赞助、捐赠或类似行为，均不当然对应游戏内道具、货币、等级、账号权益、特权、补偿、返利、兑换码、数据保留、正式版本继承或其他游戏内权益。',
      '如未来版本涉及任何付费、赞助权益、广告、商业合作、会员、充值、礼包、兑换码、虚拟货币购买或其他商业化内容，项目方将另行发布明确规则，并以届时公布的规则为准。',
    ],
  },
  {
    id: 'rmt-ban',
    badge: '04',
    title: '禁止 RMT、账号交易及违规交易',
    summary: '禁止现实货币交易、变相交易、第三方平台交易、代练代刷和资源输送。',
    items: [
      '玩家不得以任何形式进行现实货币交易，亦不得以现实货币、虚拟货币、平台积分、实物、服务、其他游戏资产、第三方平台权益或任何具有经济价值的对价，交易《摸鱼放置》内的账号、角色、金币、材料、装备、道具、资源、服务或其他游戏内权益。',
      '禁止买卖、出租、出借、转让、赠与、抵押账号；禁止买卖、出租、转让、赠与游戏内金币、材料、装备、道具、资源或其他虚拟物品。',
      '禁止通过第三方平台、聊天群、社交软件、论坛、交易网站等渠道进行游戏资产交易；禁止代练、代刷、陪练、挂机托管、资源代刷、任务代做、排行榜代打。',
      '禁止以红包、转账、礼品卡、平台余额、虚拟币、其他游戏道具等方式变相交易；禁止通过“赠送”“借用”“补偿”“赞助”“友情转移”等名义规避交易限制。',
      '项目方不承认、不保护、不恢复、不补偿任何违规交易产生的所谓权益。任何通过违规交易取得的账号、道具、货币、资源、等级、排行榜名次或其他收益，项目方均有权认定为异常收益并予以处理。',
    ],
  },
  {
    id: 'multi-account-ban',
    badge: '05',
    title: '禁止多开账号、批量账号及账号矩阵',
    summary: '禁止多开、批量注册、资源输送、账号矩阵、脚本挂机及规避处罚。',
    items: [
      '为维护测试公平性、服务器稳定性和数据有效性，玩家不得通过多开账号、批量注册、账号矩阵、脚本挂机、模拟器集群、设备伪装、IP 代理、自动化注册、自动化登录、自动化操作等方式获取不当优势。',
      '禁止使用多个账号为主账号输送金币、材料、装备、道具、资源或其他收益；禁止使用多个账号重复领取奖励、重复参与活动、重复占用排行榜或重复获取测试资源。',
      '禁止使用多个账号规避冷却时间、行动限制、产出限制、交易限制、社交限制或处罚措施。',
      '项目方有权根据登录记录、设备信息、IP 信息、操作行为、资源流向、账号关联关系、交易记录、排行榜变化、行为频率、异常收益、玩家反馈及其他合理证据，判断账号是否存在多开、批量账号、账号矩阵、资源输送、规避处罚或其他异常行为。',
      '对被认定存在上述异常行为的账号，项目方有权采取限制登录、限制注册、限制功能、回收收益、清空数据、封禁账号、封禁关联账号、限制设备、限制 IP 或其他必要措施。',
    ],
  },
  {
    id: 'data-policy',
    badge: '06',
    title: '测试数据、删档、回档与数值调整',
    summary: '测试数据不具有现实货币价值，项目方可根据需要删除、重置、修正、回档或清理数据。',
    items: [
      '测试阶段产生的所有账号数据、角色数据、等级、经验、技能、装备、材料、货币、宠物、称号、任务、地图进度、邮件、仓库、排行榜、活动记录、战斗记录、交易记录及其他游戏数据，均为测试数据。',
      '项目方有权根据测试目的、技术需要、数据质量、漏洞影响、服务器状况、安全事件、成本控制、版本规划、合规要求或其他合理原因，对全部或部分测试数据进行删除、重置、修正、回档、迁移、清理或停止保存。',
      '即使当前测试阶段允许公开访问和注册，也不代表测试数据会永久保留。',
      '项目方有权在不另行补偿的情况下，对游戏数值和系统进行调整，包括经验、掉率、金币产出、装备属性、怪物强度、技能倍率、行动时间、强化概率、合成概率、任务奖励、商店价格、排行榜规则、战斗机制、资源产出、资源消耗、地图难度等。',
      '如因漏洞、异常配置、服务器错误、结算错误、攻击行为、第三方服务异常或其他原因导致异常收益，项目方有权回收异常收益、扣除异常货币、删除异常道具、回退异常等级、修正排行榜、撤销奖励、限制账号或进行局部/全服回档。',
    ],
  },
  {
    id: 'conduct',
    badge: '07',
    title: '玩家行为规范',
    summary: '禁止外挂、脚本、漏洞利用、攻击服务器、违法内容、骚扰、诈骗、广告引流等行为。',
    items: [
      '玩家不得使用外挂、脚本、自动化工具、模拟点击工具、加速器作弊模块、封包工具、内存修改器、接口调用工具、机器人程序或其他影响测试公平性、服务器稳定性或正常游戏秩序的工具。',
      '玩家不得利用漏洞、程序错误、服务器延迟、接口缺陷、结算异常、重复领取、异常掉落、异常交易、异常战斗结果等方式获取不当利益。',
      '玩家不得攻击服务器、扫描接口、绕过登录验证、绕过风控系统、批量请求接口、恶意压测、反编译、破解、篡改客户端、篡改通信数据或干扰项目正常运行。',
      '玩家不得发布违法违规、色情低俗、暴力恐怖、赌博诈骗、违禁品、仇恨攻击、极端主义、侵害未成年人权益、侵犯他人隐私、侵犯知识产权或其他不适当内容。',
      '玩家不得侮辱、诽谤、骚扰、威胁、恶意曝光他人隐私、人肉搜索、煽动网暴、组织攻击玩家、攻击管理员、攻击开发者或破坏社区秩序。',
    ],
  },
  {
    id: 'ban-policy',
    badge: '08',
    title: '违规处理、账号封禁与关联处罚',
    summary: '明确项目方可对违规账号采取警告、限制、冻结、永久封禁、清空数据和关联处罚。',
    items: [
      '玩家违反法律法规、本守则、项目方公告、游戏规则、社区规则、测试规则，或实施破坏测试公平性、服务器稳定性、数据真实性、社区秩序、其他玩家权益的行为，项目方有权采取一项或多项处理措施。',
      '处理措施包括提醒、警告、删除违规内容、禁言、限制聊天、限制交易、限制邮件、限制排行榜、限制活动参与、限制登录、限制注册、限制设备、限制 IP、回收异常收益、删除异常道具、扣除异常货币、回退等级、重置角色、重置账号数据、修正排行榜、取消奖励、临时冻结账号、长期冻结账号、永久封禁账号、封禁或限制关联账号等。',
      '对于外挂脚本、攻击服务器、漏洞大量获利、传播漏洞方法、现实货币交易、账号交易、虚拟物品交易、代练代刷、多开账号、账号矩阵、资源输送、规避处罚、诈骗、盗号、冒充项目方等严重违规行为，项目方有权不经事先警告直接采取永久封禁账号、清空数据、回收收益、取消奖励、封禁关联账号、限制设备或限制 IP 等处理措施。',
      '如多个账号存在相同或高度相似的设备环境、IP、网络环境、浏览器环境、登录凭证、资源输送、收益转移、协同刷取、规避限制、共同参与违规或其他关联特征，项目方有权认定为关联账号并进行关联处理。',
      '为保护风控系统、安全机制和其他玩家权益，项目方无需向违规玩家完整披露检测规则、风控模型、判定细节、日志内容或其他可能被用于规避检测的信息。',
    ],
  },
  {
    id: 'privacy',
    badge: '09',
    title: '个人信息与隐私',
    summary: '说明账号注册、登录验证、安全风控、反作弊、反馈处理所需的信息处理规则。',
    items: [
      '项目方可能基于账号注册、登录验证、测试体验、安全风控、反作弊、客服处理、问题反馈、数据统计、服务器维护、异常排查、违规处理等目的，处理必要的用户信息。',
      '可能处理的信息包括账号信息、昵称、邮箱、登录记录、设备信息、浏览器信息、网络信息、IP 地址、游戏行为数据、反馈内容、违规记录、客服沟通记录、错误日志及其他为实现测试目的所必要的信息。',
      '项目方将尽量遵循合法、正当、必要、诚信、最小必要原则处理用户信息，并采取合理安全措施防止信息泄露、篡改、丢失或被非法使用。',
      '项目方不会主动要求玩家提供身份证号、银行卡号、支付密码、验证码、家庭住址等与当前测试无关的敏感信息。玩家也不应在聊天、昵称、社区、反馈渠道或其他公开场景中主动披露上述信息。',
      '玩家可通过联系邮箱联系项目方，申请查询、更正、删除相关个人信息，或申请注销账号。因安全风控、争议处理、数据备份、日志留存、法律义务或其他合理原因需要保留的信息除外。',
    ],
  },
  {
    id: 'disclaimer',
    badge: '10',
    title: '免责声明与责任限制',
    summary: '测试阶段不保证持续可用、稳定、无错误、无漏洞、无数据丢失或符合玩家个人预期。',
    items: [
      '在法律允许的范围内，项目方不对《摸鱼放置》测试版本的持续可用性、稳定性、准确性、完整性、安全性、兼容性、无错误、无漏洞、无中断、无数据丢失或符合玩家个人预期作出保证。',
      '因测试阶段固有的不稳定性导致的卡顿、掉线、无法登录、数据异常、奖励异常、排行榜变化、数值调整、功能关闭、临时维护、回档、删档、停止服务、终止测试等，项目方将在合理范围内处理，但不承诺赔偿或补偿。',
      '因玩家自身原因导致的损失，包括账号共享、泄露密码、相信诈骗、私下交易、现实货币交易、账号交易、资源交易、访问非官方链接、下载非官方客户端、使用外挂、违反规则被处罚、设备故障、网络问题、误操作、主动公开个人信息等，由玩家自行承担。',
      '因第三方服务、网络环境、浏览器、操作系统、云服务、邮件服务、安全服务、社区平台、网络运营商、黑客攻击、DDoS 攻击、自然灾害、政府行为、法律法规变化、不可抗力或其他非项目方可完全控制的原因导致的损失，项目方将在合理范围内协助处理，但不承担超出法律规定的责任。',
      '本守则中的免责和责任限制条款，不排除或限制用户依法享有的强制性法定权利，也不免除项目方因故意、重大过失、违法处理个人信息或其他依法不得免除的责任。',
    ],
  },
]

function emitAccept() {
  if (!accepted.value) return
  emit('accept')
}

const InfoCard = defineComponent({
  name: 'InfoCard',
  props: {
    title: {
      type: String,
      required: true,
    },
    content: {
      type: String,
      required: true,
    },
  },
  template: `
    <div class="rounded-3xl border border-slate-200 bg-white p-5 shadow-sm">
      <h3 class="text-base font-bold text-slate-950">{{ title }}</h3>
      <p class="mt-2 text-sm leading-6 text-slate-600">{{ content }}</p>
    </div>
  `,
})
</script>
