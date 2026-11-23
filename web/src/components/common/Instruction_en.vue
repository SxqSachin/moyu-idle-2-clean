<template>
  <section class="min-h-screen bg-slate-50 text-slate-900">
    <div class="mx-auto flex max-w-7xl flex-col gap-8 px-4 py-8 sm:px-6 lg:flex-row lg:px-8">
      <!-- Sidebar -->
      <aside class="lg:sticky lg:top-6 lg:h-[calc(100vh-3rem)] lg:w-80 lg:shrink-0">
        <div class="rounded-3xl border border-slate-200 bg-white p-5 shadow-sm">
          <div class="flex items-center gap-3">
            <div class="flex h-12 w-12 items-center justify-center rounded-2xl bg-slate-900 text-lg font-bold text-white">
              MI
            </div>
            <div>
              <p class="text-sm font-medium text-slate-500">Player Rules</p>
              <h1 class="text-xl font-bold tracking-tight">{{ projectName }}</h1>
            </div>
          </div>

          <!-- <div class="mt-5 space-y-2 rounded-2xl bg-slate-50 p-4 text-sm text-slate-600">
            <p><span class="font-semibold text-slate-800">Version:</span> {{ version }}</p>
            <p><span class="font-semibold text-slate-800">Effective:</span> {{ effectiveDate }}</p>
            <p><span class="font-semibold text-slate-800">Operator:</span> {{ operatorName }}</p>
            <p><span class="font-semibold text-slate-800">Contact:</span> {{ contactEmail }}</p>
          </div> -->

          <nav class="mt-6 max-h-[42vh] space-y-1 overflow-y-auto pr-1">
            <a
              v-for="section in sections"
              :key="section.id"
              :href="`#${section.id}`"
              class="block rounded-xl px-3 py-2 text-sm font-medium text-slate-600 transition hover:bg-slate-100 hover:text-slate-950"
            >
              {{ section.shortTitle }}
            </a>
          </nav>
        </div>
      </aside>

      <!-- Main -->
      <main class="min-w-0 flex-1">
        <header class="overflow-hidden rounded-3xl border border-slate-200 bg-white shadow-sm">
          <div class="border-b border-slate-200 bg-slate-900 p-8 text-white sm:p-10">
            <p class="text-sm font-semibold uppercase tracking-[0.25em] text-slate-300">
              Early Testing Version
            </p>
            <h2 class="mt-4 text-3xl font-black tracking-tight sm:text-5xl">
              {{ projectName }} Player Rules & Disclaimer
            </h2>
            <p class="mt-5 max-w-3xl text-base leading-7 text-slate-300 sm:text-lg">
              Please read this document carefully before registering, logging in, accessing,
              playing, submitting feedback, or joining any related community for {{ projectName }}.
            </p>
          </div>

          <div class="grid gap-4 p-6 sm:grid-cols-3 sm:p-8">
            <div
              v-for="card in highlightCards"
              :key="card.title"
              class="rounded-2xl border border-slate-200 bg-slate-50 p-5"
            >
              <p class="text-sm font-bold text-slate-950">{{ card.title }}</p>
              <p class="mt-2 text-sm leading-6 text-slate-600">{{ card.body }}</p>
            </div>
          </div>
        </header>

        <div class="mt-8 rounded-3xl border border-amber-200 bg-amber-50 p-6 text-amber-950 shadow-sm">
          <div class="flex flex-col gap-3 sm:flex-row sm:items-start">
            <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl bg-amber-200 font-bold">
              !
            </div>
            <div>
              <h3 class="text-lg font-bold">Important Notice</h3>
              <p class="mt-2 leading-7">
                {{ projectName }} is currently an early testing version for personal learning,
                technical validation, and project showcase purposes. Test data has no real-world
                monetary value and is not guaranteed to be permanently retained.
              </p>
            </div>
          </div>
        </div>

        <section class="mt-8 space-y-4">
          <article
            v-for="section in sections"
            :id="section.id"
            :key="section.id"
            class="scroll-mt-8 overflow-hidden rounded-3xl border border-slate-200 bg-white shadow-sm"
          >
            <div
              class="flex w-full items-start justify-between gap-4 p-6 text-left sm:p-7"
            >
              <div>
                <p class="text-xs font-bold uppercase tracking-[0.2em] text-slate-400">
                  Article {{ section.article }}
                </p>
                <h3 class="mt-2 text-xl font-black tracking-tight text-slate-950 sm:text-2xl">
                  {{ section.title }}
                </h3>
                <p class="mt-2 text-sm leading-6 text-slate-500">
                  {{ section.summary }}
                </p>
              </div>
            </div>

            <div class="border-t border-slate-200 p-6 sm:p-7">
              <ol class="space-y-4">
                <li
                  v-for="(item, index) in section.items"
                  :key="index"
                  class="rounded-2xl bg-slate-50 p-4 text-sm leading-7 text-slate-700 sm:text-base"
                >
                  <div class="flex gap-3">
                    <span class="mt-1 flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-slate-900 text-xs font-bold text-white">
                      {{ index + 1 }}
                    </span>
                    <div class="min-w-0">
                      <p>{{ item.text }}</p>
                      <ul v-if="item.children?.length" class="mt-3 list-disc space-y-2 pl-6 text-slate-600">
                        <li v-for="child in item.children" :key="child">{{ child }}</li>
                      </ul>
                    </div>
                  </div>
                </li>
              </ol>
            </div>
          </article>
        </section>

        <footer class="py-8 text-center text-sm text-slate-500">
          © {{ new Date().getFullYear() }} {{ projectName }}. Testing version only.
        </footer>
      </main>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'

type RuleItem = {
  text: string
  children?: string[]
}

type RuleSection = {
  id: string
  article: string
  title: string
  shortTitle: string
  summary: string
  items: RuleItem[]
}

type HighlightCard = {
  title: string
  body: string
}

type Acknowledgement = {
  id: string
  text: string
}

const props = withDefaults(defineProps<{
  projectName?: string
  version?: string
  effectiveDate?: string
  operatorName?: string
  contactEmail?: string
  applicableLaw?: string
}>(), {
  projectName: 'Moyu Idle',
  version: 'V1.0',
  effectiveDate: 'YYYY-MM-DD',
  operatorName: 'Project Owner',
  contactEmail: 'contact@example.com',
  applicableLaw: 'the applicable laws of the competent jurisdiction',
})

const emit = defineEmits<{
  accept: []
}>()

const checked = ref<string[]>([])

const highlightCards: HighlightCard[] = [
  {
    title: 'Testing Version',
    body: 'The current version is for early testing, personal learning, technical validation, and project showcase purposes.',
  },
  {
    title: 'No Real-World Value',
    body: 'Accounts, characters, items, currencies, rankings, rewards, and all test data have no real-world monetary value.',
  },
  {
    title: 'Strict Enforcement',
    body: 'Cheating, RMT, account trading, multi-account abuse, botting, exploit abuse, and disruptive conduct may lead to account bans.',
  },
]

const sections: RuleSection[] = [
  {
    id: 'project-nature',
    article: '1',
    title: 'Project Nature',
    shortTitle: 'Project Nature',
    summary: 'Clarifies that the project is an early testing version for learning, validation, and showcase purposes.',
    items: [
      {
        text: `${props.projectName} is currently in an early testing stage. The project is primarily intended for personal learning, technical validation, project showcase, gameplay experimentation, server stability testing, balance testing, user experience collection, and other non-final-release purposes.`,
      },
      {
        text: `${props.projectName} is not a final release version. It does not constitute any promise by the project owner regarding a final version, long-term service, continuous operation, commercial launch, data retention, feature retention, future updates, or ongoing maintenance.`,
      },
      {
        text: 'The current testing version may contain unfinished content, bugs, balance issues, unstable servers, missing features, incorrect text, interface errors, data anomalies, compatibility issues, and other issues commonly found during testing.',
      },
      {
        text: 'You understand and agree that the project owner may adjust, suspend, shut down, reset, roll back, or terminate all or part of the service at any time for learning, testing, showcase, technical adjustment, cost control, security, compliance, or other reasonable reasons.',
      },
      {
        text: 'Public access or account registration during testing does not mean that the project has entered final release, formal operation, commercial operation, or a long-term stable service stage.',
      },
    ],
  },
  {
    id: 'scope',
    article: '2',
    title: 'Scope of These Rules',
    shortTitle: 'Scope',
    summary: 'Defines where and when these rules apply.',
    items: [
      {
        text: `These rules apply to your access to, registration for, login to, gameplay within, feedback submission for, and interaction with ${props.projectName}, including related pages, servers, account systems, game content, communities, feedback channels, announcement channels, and related services.`,
      },
      {
        text: 'These rules, together with any announcements, privacy notices, event rules, update notes, enforcement notices, and other rules published by the project owner, form the basic rules governing your use of the project.',
      },
      {
        text: 'If separate rules are later published for a specific matter and conflict with these rules, the separate rules shall apply to that specific matter, while these rules continue to apply to all other matters.',
      },
    ],
  },
  {
    id: 'account',
    article: '3',
    title: 'Registration, Login, and Account Rules',
    shortTitle: 'Accounts',
    summary: 'Explains account responsibilities, security, and restrictions.',
    items: [
      {
        text: 'Players may register an account through the official entry provided by the project owner and access the testing version.',
      },
      {
        text: 'You must ensure that your registration, login, and usage are truthful, lawful, and compliant. You may not register or log in using false information, another person’s information, illegally obtained information, or methods that violate applicable laws, platform rules, or these rules.',
      },
      {
        text: 'You are responsible for properly safeguarding your account, password, verification codes, email, third-party login credentials, devices, browser sessions, and other login credentials.',
      },
      {
        text: 'Losses caused by your own disclosure of account information, account sharing, weak passwords, unofficial links, unofficial clients, scams, or allowing others to use your account shall be borne by you.',
      },
      {
        text: 'You may not sell, rent, transfer, share, gift, pledge, trade, power-level, boost, batch-register, maliciously register, script-register, credential-stuff, steal, impersonate, or otherwise abuse accounts.',
      },
      {
        text: 'The project owner may restrict, suspend, freeze, or ban accounts, or restrict registration, login, chat, rankings, trading, social features, mail, grouping, and other functions based on server capacity, abnormal registrations, account security, risk control, testing needs, maintenance, violations, or other reasonable reasons.',
      },
      {
        text: 'Accounts are used only for participating in the current testing experience. Successful account registration does not mean that the project owner promises to permanently retain the account, data, nickname, character, progress, or any other interest.',
      },
    ],
  },
  {
    id: 'capacity',
    article: '4',
    title: 'Testing Capacity and Access Restrictions',
    shortTitle: 'Capacity',
    summary: 'Allows the project owner to limit access, close registration, queue logins, or suspend service when needed.',
    items: [
      {
        text: `${props.projectName} is currently a testing version. Server capacity, bandwidth, database capacity, concurrency capability, maintenance capacity, customer support capacity, and issue-handling capacity may be limited.`,
      },
      {
        text: 'When server load, abnormal access, malicious registrations, scripted behavior, attacks, major bugs, data anomalies, third-party service issues, maintenance needs, cost pressure, or other factors affect stability, the project owner may take necessary measures.',
      },
      {
        text: 'Necessary measures may include closing registration, limiting login volume, enabling login queues, limiting registrations by device or IP, restricting certain network environments, suspending features, temporary downtime, emergency maintenance, data rollback, data cleanup, version rollback, server shutdown, or termination of the test.',
      },
      {
        text: 'Such measures are reasonable adjustments during testing and do not create any obligation to compensate players for testing progress, game data, virtual items, ranking positions, event rewards, or any other testing results.',
      },
    ],
  },
  {
    id: 'non-commercial',
    article: '5',
    title: 'Non-Commercial Testing Statement',
    shortTitle: 'Non-Commercial',
    summary: 'States that the current testing version does not provide in-game paid services.',
    items: [
      {
        text: `${props.projectName} currently does not provide in-game top-up, in-app purchase, paid purchase, paid draw, paid acceleration, paid unlock, rewarded advertising, virtual currency purchase, virtual item cash purchase, or other in-game commercial paid services during the testing stage.`,
      },
      {
        text: 'In-game coins, materials, equipment, items, pets, skills, experience, levels, titles, ranking rewards, event rewards, test rewards, and other virtual content are used only for testing and have no real-world monetary value.',
      },
      {
        text: 'No in-game virtual item, virtual currency, account data, character data, ranking data, or test data may be exchanged for cash, physical goods, securities, points, platform benefits, third-party benefits, or any real-world return.',
      },
      {
        text: 'Players may not buy, sell, rent, transfer, pledge, gift, boost, power-level, trade accounts, characters, virtual currency, virtual items, game resources, ranking positions, or other test data in any form.',
      },
      {
        text: 'Unless expressly announced by the project owner, any individual, group, website, social account, or third-party platform claiming to sell accounts, gift packs, activation codes, redemption codes, items, currency, internal permissions, administrator privileges, special access, or data retention rights is unrelated to the project owner.',
      },
      {
        text: 'Losses caused by private transactions, transfers, purchases of accounts or resources, reliance on third-party promises, or participation in unofficial activities shall be borne by the player. The project owner may assist in reasonable verification but does not promise recovery, restoration, or compensation.',
      },
      {
        text: 'Unless expressly stated in writing by the project owner, any off-game voluntary support, tip, sponsorship, donation, or similar act does not automatically correspond to any in-game item, currency, level, account interest, privilege, compensation, rebate, redemption code, data retention, final-version inheritance, or other in-game benefit.',
      },
      {
        text: 'If any future version involves paid features, sponsorship benefits, advertising, commercial cooperation, membership, top-up, gift packs, redemption codes, virtual currency purchases, or other commercial content, the project owner will publish separate rules, and those rules shall apply.',
      },
    ],
  },
  {
    id: 'rmt',
    article: '6',
    title: 'No RMT, Account Trading, or Prohibited Player Trading',
    shortTitle: 'No RMT',
    summary: 'Prohibits real-money trading, account trading, power leveling, resource selling, and disguised player trading.',
    items: [
      {
        text: 'Accounts, characters, levels, experience, coins, materials, equipment, items, pets, skills, titles, ranking positions, event rewards, test rewards, and all other in-game data during testing are only for testing and have no real-world monetary value.',
      },
      {
        text: `Players may not engage in real-money trading. Players may not use cash, virtual currency, platform points, physical goods, services, assets from other games, third-party platform benefits, or anything of economic value to trade accounts, characters, coins, materials, equipment, items, resources, services, or other in-game interests in ${props.projectName}.`,
      },
      {
        text: 'The following player-to-player activities are prohibited:',
        children: [
          'Buying, selling, renting, lending, transferring, gifting, or pledging accounts.',
          'Buying, selling, renting, transferring, or gifting in-game coins, materials, equipment, items, resources, or other virtual items.',
          'Purchasing or selling in-game resources, items, accounts, characters, ranking positions, or test data with real-world money.',
          'Trading game assets through third-party platforms, chat groups, social media, forums, transaction websites, or similar channels.',
          'Power leveling, boosting, paid carry services, account hosting, resource farming, task completion services, or ranking boosting.',
          'Using red packets, transfers, gift cards, platform balance, cryptocurrency, items from other games, or similar methods to disguise trading.',
          'Using labels such as “gift,” “borrow,” “compensation,” “sponsorship,” or “friendly transfer” to circumvent trading restrictions.',
          'Using multiple accounts to transfer resources, funnel benefits, bypass restrictions, manipulate rankings, or harm testing balance.',
          'Organizing, brokering, advertising, introducing, guaranteeing, collecting payment for, delivering, or assisting with any of the above transactions.',
        ],
      },
      {
        text: `Players may not publish, imply, guide, advertise, or promote account trading, resource trading, boosting, botting, private payment, third-party trading groups, or similar information in in-game chat, nicknames, signatures, communities, comments, feedback channels, or any other ${props.projectName}-related context.`,
      },
      {
        text: 'Losses, scams, data anomalies, account bans, or other disputes arising from RMT, account trading, private trading, third-party trading, boosting, or other prohibited transactions shall be borne by the player.',
      },
      {
        text: 'The project owner does not recognize, protect, restore, or compensate any claimed rights arising from prohibited transactions. Any account, item, currency, resource, level, ranking position, or other benefit obtained through prohibited transactions may be deemed abnormal gains and handled accordingly.',
      },
    ],
  },
  {
    id: 'multi-account',
    article: '7',
    title: 'No Multi-Account Abuse, Mass Accounts, or Account Networks',
    shortTitle: 'No Multi-Account Abuse',
    summary: 'Prohibits multi-account abuse, resource funneling, bot farms, emulator clusters, and punishment evasion.',
    items: [
      {
        text: 'To maintain testing fairness, server stability, and data validity, players may not gain improper advantages through multi-accounting, mass registration, account networks, scripted idling, emulator clusters, device spoofing, IP proxies, automated registration, automated login, or automated operation.',
      },
      {
        text: 'Unless expressly permitted by the project owner, players may not engage in the following conduct:',
        children: [
          'Registering, controlling, or using a large number of accounts to participate in testing.',
          'Using multiple accounts to funnel coins, materials, equipment, items, resources, or other benefits to a main account.',
          'Using multiple accounts to repeatedly claim rewards, repeatedly join events, occupy rankings, or obtain testing resources.',
          'Using multiple accounts to bypass cooldowns, action limits, production limits, trading limits, social limits, or enforcement measures.',
          'Using multiple accounts to maliciously occupy nicknames, resource points, market positions, rankings, or server capacity.',
          'Using emulators, multiple windows, virtual machines, cloud phones, scripts, auto-clickers, or API tools to operate accounts in bulk.',
          'Using proxy IPs, device fingerprint spoofing, browser environment spoofing, or similar methods to bypass risk controls.',
          'Continuing to register or use other accounts to evade punishment after an account has been sanctioned.',
          'Organizing others to mass-register, mass-login, mass-idle, mass-farm resources, or assist with account network operations.',
        ],
      },
      {
        text: 'The project owner may determine whether an account involves multi-accounting, mass accounts, account networks, resource funneling, punishment evasion, or other abnormal behavior based on login records, device information, IP information, behavior records, resource flows, account associations, trading records, ranking changes, behavior frequency, abnormal gains, player reports, and other reasonable evidence.',
      },
      {
        text: 'For accounts deemed to involve multi-accounting, account networks, resource funneling, mass registration, scripted idling, malicious resource occupation, or punishment evasion, the project owner may restrict login, restrict registration, restrict features, recover gains, clear data, ban accounts, ban related accounts, restrict devices, restrict IPs, or take other necessary measures.',
      },
      {
        text: 'Players may not use arguments such as “the accounts are operated by different people,” “the accounts belong to relatives or friends,” “it is only testing,” “there was no direct RMT,” or “there was no clear profit” to challenge reasonable handling based on abnormal behavior and account association evidence.',
      },
    ],
  },
  {
    id: 'data',
    article: '8',
    title: 'Test Data, Wipes, Rollbacks, and Adjustments',
    shortTitle: 'Test Data',
    summary: 'Allows test data deletion, resets, corrections, rollbacks, migrations, and balance changes.',
    items: [
      {
        text: 'All account data, character data, levels, experience, skills, equipment, materials, currency, pets, titles, tasks, map progress, mail, storage, rankings, event records, combat records, transaction records, and other game data generated during testing are test data.',
      },
      {
        text: 'The project owner may delete, reset, correct, roll back, migrate, clean up, or stop retaining all or part of the test data for testing purposes, technical needs, data quality, bug impact, server conditions, security incidents, cost control, version planning, compliance requirements, or other reasonable reasons.',
      },
      {
        text: 'Even if public access and registration are allowed during the current testing stage, this does not mean that test data will be permanently retained.',
      },
      {
        text: 'The project owner may adjust game values and systems without separate compensation, including experience, drop rates, coin output, equipment attributes, monster difficulty, skill multipliers, action durations, enhancement rates, crafting rates, task rewards, shop prices, ranking rules, combat mechanisms, resource output, resource consumption, and map difficulty.',
      },
      {
        text: 'Value adjustments may affect your strategy, efficiency, character strength, item value, ranking position, or gameplay experience. You understand and agree that such adjustments are part of the testing process and do not create a compensation obligation.',
      },
      {
        text: 'If abnormal gains arise from bugs, misconfiguration, server errors, settlement errors, attacks, third-party service issues, or other reasons, the project owner may recover abnormal gains, deduct abnormal currency, delete abnormal items, roll back abnormal levels, correct rankings, revoke rewards, restrict accounts, or perform partial/full rollbacks.',
      },
      {
        text: 'Players may not demand retention of abnormal data or compensation on the grounds that they have already obtained, used, traded, ranked with, or spent time on such data.',
      },
    ],
  },
  {
    id: 'conduct',
    article: '9',
    title: 'Player Conduct',
    shortTitle: 'Conduct',
    summary: 'Sets baseline conduct rules for gameplay, communication, security, and community behavior.',
    items: [
      {
        text: 'You must comply with applicable laws, public order and morality, platform rules, these rules, and any rules published by the project owner while using the project.',
      },
      {
        text: 'You may not use cheats, scripts, automation tools, auto-clickers, speed-modifying modules, packet tools, memory editors, API tools, bots, or any other tool that affects testing fairness, server stability, or normal gameplay order.',
      },
      {
        text: 'You may not exploit bugs, program errors, server delays, API defects, settlement anomalies, duplicate claims, abnormal drops, abnormal trades, abnormal combat results, or similar issues to gain improper benefits.',
      },
      {
        text: 'You may not spread exploit details, organize others to use exploits, sell exploit methods, publicly share abnormal farming methods, or induce others to disrupt testing order.',
      },
      {
        text: 'You may not attack servers, scan APIs, bypass login verification, bypass risk controls, issue bulk API requests, maliciously stress-test the service, reverse engineer, crack, modify clients, tamper with communication data, or interfere with normal project operation.',
      },
      {
        text: 'You may not publish illegal, pornographic, violent, terrorist, gambling, scam, prohibited-goods, hateful, extremist, minor-harming, privacy-infringing, intellectual-property-infringing, or otherwise inappropriate content.',
      },
      {
        text: 'You may not insult, defame, harass, threaten, dox, incite harassment, organize attacks against players, administrators, developers, or disrupt community order.',
      },
      {
        text: 'You may not advertise, redirect traffic, post scam links, phishing links, cheat advertisements, boosting advertisements, account trading advertisements, fake benefit campaigns, fake announcements, or other disruptive information.',
      },
      {
        text: 'You may not impersonate the project owner, administrators, customer support, moderators, developers, partners, streamers, or any other special identity.',
      },
      {
        text: 'You may not spam, file malicious reports, submit malicious complaints, spread unverified information, create panic, organize disruption, incite server attacks, or attack support channels.',
      },
    ],
  },
  {
    id: 'bugs',
    article: '10',
    title: 'Bug Reports and Abnormal Handling',
    shortTitle: 'Bugs',
    summary: 'Encourages responsible reporting and prohibits exploit abuse.',
    items: [
      {
        text: 'Players who discover bugs, abnormal gains, balance errors, item duplication, duplicate claims, abnormal drops, abnormal experience, abnormal currency, abnormal settlements, API problems, or other anomalies should report them through the designated channel.',
      },
      {
        text: 'Players may not profit from bugs, publicly disclose exploit details, teach others to reproduce bugs, organize others to exploit bugs, or assist others in exploit abuse.',
      },
      {
        text: 'For players who actively report major issues without malicious exploitation, the project owner may, at its discretion, provide non-cash testing commemorations, titles, acknowledgements, or other test rewards.',
      },
      {
        text: 'For players who knowingly and repeatedly exploit, spread, sell, or organize exploitation of anomalies, the project owner may issue warnings, mute accounts, restrict features, freeze accounts, recover abnormal gains, reset data, cancel rewards, ban accounts, or remove such players from communities.',
      },
      {
        text: 'The project owner may determine bugs, abnormal gains, abnormal data, and violations based on server logs, operation records, login records, API records, transaction records, chat records, device information, IP information, behavior patterns, player reports, and other evidence.',
      },
    ],
  },
  {
    id: 'community',
    article: '11',
    title: 'Community, Chat, and User Content',
    shortTitle: 'Community',
    summary: 'Covers player-generated content, feedback, screenshots, videos, and community conduct.',
    items: [
      {
        text: `Content you publish in in-game chat, feedback channels, comments, communities, groups, forums, or any other ${props.projectName}-related context must be lawful, truthful, appropriate, and must not infringe the rights of others.`,
      },
      {
        text: 'Nicknames, avatars, signatures, chat content, feedback, screenshots, videos, guides, suggestions, comments, fan content, or other user-generated content may not contain illegal, infringing, insulting, harassing, scam, advertising, pornographic, violent, terrorist, gambling, cheat-related, traffic-redirecting, malicious, or otherwise inappropriate content.',
      },
      {
        text: 'You warrant that content you submit or publish does not infringe any third-party copyright, trademark, portrait right, reputation right, privacy right, personal information right, or other lawful right.',
      },
      {
        text: 'You agree that the project owner may use your submitted feedback, suggestions, screenshots, nicknames, issue descriptions, and game records within a reasonable scope for testing, showcase, feedback organization, issue diagnosis, project records, public explanations, community management, and enforcement.',
      },
      {
        text: 'The project owner may delete, hide, collapse, restrict display, restrict posting, mute, ban, or remove user content that violates these rules or is unsuitable for display.',
      },
      {
        text: 'You are responsible for disputes, complaints, enforcement, or losses caused by content you publish. If the project owner suffers losses as a result, the project owner may seek corresponding responsibility from you.',
      },
    ],
  },
  {
    id: 'ip',
    article: '12',
    title: 'Intellectual Property',
    shortTitle: 'IP',
    summary: 'Protects game content, code, assets, systems, text, icons, audio, and related materials.',
    items: [
      {
        text: `Unless otherwise stated, program code, interfaces, characters, items, maps, system designs, text, icons, sound effects, music, data tables, database structures, server architecture, names, marks, assets, and other content related to ${props.projectName} are owned by the project owner or the relevant rights holders.`,
      },
      {
        text: `Without permission from the project owner or relevant rights holder, players may not copy, distribute, modify, adapt, disassemble, decompile, reverse engineer, scrape, export, sell, rent, publish, or commercially use any ${props.projectName} content.`,
      },
      {
        text: 'Players may not create, distribute, sell, or provide cheats, private servers, cracked versions, modified versions, mirror sites, unofficial clients, unofficial APIs, automation tools, or other content or services that infringe rights or disrupt testing order.',
      },
      {
        text: 'Players may publish reasonable screenshots, videos, guides, reviews, feedback, or fan content for personal non-commercial purposes, provided that such use does not violate laws, these rules, or other project rules.',
      },
      {
        text: 'If the project owner believes such content is infringing, misleading, unlawful, disruptive, harmful to project reputation, or otherwise inappropriate, the project owner may require deletion, modification, or cessation of distribution.',
      },
    ],
  },
  {
    id: 'minors',
    article: '13',
    title: 'Protection of Minors',
    shortTitle: 'Minors',
    summary: 'Requires minors to use the project with guardian knowledge and guidance.',
    items: [
      {
        text: 'Minors should access, register, log in to, or use the project only with the knowledge, consent, and guidance of their guardians.',
      },
      {
        text: 'If you are a minor, you should participate in testing only after your guardian is fully informed and consents. You may not impersonate an adult, use another person’s account, or bypass any protection measures for minors.',
      },
      {
        text: 'Guardians should reasonably guide and supervise minors in participating in online games or testing experiences to avoid addiction, excessive use, inappropriate consumption, social risks, or exposure to unsuitable content.',
      },
      {
        text: 'The project owner may take necessary restrictions on suspected minor accounts based on laws, platform rules, risk control, or actual project conditions, including limiting login, limiting time, limiting social features, limiting certain functions, requiring additional verification, or closing accounts.',
      },
      {
        text: 'Players may not induce, deceive, or coerce minors into providing personal information, engaging in private transactions, joining unofficial groups, bypassing protection measures, accessing inappropriate content, or engaging in conduct that may harm minors.',
      },
    ],
  },
  {
    id: 'privacy',
    article: '14',
    title: 'Personal Information and Privacy',
    shortTitle: 'Privacy',
    summary: 'Explains what information may be processed and for what purposes.',
    items: [
      {
        text: 'The project owner may process necessary user information for account registration, login verification, testing experience, security and risk control, anti-cheat, support handling, feedback, statistics, server maintenance, anomaly investigation, and enforcement.',
      },
      {
        text: 'Information that may be processed includes account information, nickname, email, login records, device information, browser information, network information, IP address, gameplay behavior data, feedback content, violation records, support communications, error logs, and other information necessary to achieve testing purposes.',
      },
      {
        text: 'The project owner will strive to process user information according to principles of lawfulness, legitimacy, necessity, good faith, and data minimization, and will take reasonable security measures to prevent leakage, alteration, loss, or unlawful use.',
      },
      {
        text: 'The project owner will not actively request sensitive information unrelated to the current test, such as ID numbers, bank card numbers, payment passwords, verification codes, or home addresses. Players should not disclose such information in chat, nicknames, communities, feedback channels, or other public contexts.',
      },
      {
        text: `Players may contact the project owner at ${props.contactEmail} to request access, correction, deletion of relevant personal information, or account cancellation, except where retention is necessary for security, dispute handling, backups, logs, legal obligations, or other reasonable reasons.`,
      },
      {
        text: 'Losses caused by players publicly disclosing personal information, revealing account information, clicking unofficial links, using unofficial clients, or relying on third-party scams shall be borne by the player.',
      },
      {
        text: 'The project owner may update personal information handling rules based on project adjustments, technical changes, legal requirements, or service needs, and may notify players in a reasonable manner.',
      },
    ],
  },
  {
    id: 'third-party',
    article: '15',
    title: 'Third-Party Services and External Links',
    shortTitle: 'Third Parties',
    summary: 'Limits responsibility for cloud services, networks, browsers, platforms, communities, and external links.',
    items: [
      {
        text: `${props.projectName} may use third-party services such as servers, cloud services, CDN, databases, email, login services, analytics, security services, communities, browsers, operating systems, and network operators.`,
      },
      {
        text: 'If third-party service anomalies, maintenance, restrictions, failures, shutdowns, policy changes, network fluctuations, security incidents, or force majeure cause login failures, delays, data anomalies, email failures, service interruptions, access failures, or other issues, the project owner may assist within a reasonable scope but does not promise liability beyond what is legally required.',
      },
      {
        text: `Players who access third-party websites, groups, tools, payment pages, or external links through ${props.projectName}-related pages or communities should assess the risks themselves.`,
      },
      {
        text: 'External links, download addresses, transaction channels, payment channels, benefit pages, support accounts, or community accounts not expressly published or endorsed by the project owner do not represent the project owner. Losses resulting from such sources shall be borne by the player.',
      },
    ],
  },
  {
    id: 'region',
    article: '16',
    title: 'Regions and Access',
    shortTitle: 'Regions',
    summary: 'Explains that service may not be available or lawful in every region.',
    items: [
      {
        text: `${props.projectName} is currently a testing version. The project owner may not provide stable, continuous, or official services in all countries or regions.`,
      },
      {
        text: 'Players should confirm whether the laws, platform rules, and network service rules of their location permit access, registration, download, use, or participation in the testing experience.',
      },
      {
        text: 'If the laws, platform rules, or network service rules of your location do not allow participation in this test, you should immediately stop accessing and using the project.',
      },
      {
        text: 'The project owner may restrict access by certain regions, network environments, devices, or accounts based on laws, platform requirements, server load, security controls, testing objectives, or other reasonable reasons.',
      },
      {
        text: 'Multilingual content in the testing version is provided only to improve understanding and experience for users of different languages. It does not represent a formal launch, commercial operation, long-term service, or continuous maintenance commitment in any specific country or region.',
      },
    ],
  },
  {
    id: 'service',
    article: '17',
    title: 'Service Interruptions, Maintenance, and Termination',
    shortTitle: 'Service',
    summary: 'Allows maintenance, downtime, rollback, deletion, suspension, and termination.',
    items: [
      {
        text: 'During testing, temporary maintenance, emergency downtime, version updates, server migration, database maintenance, security fixes, configuration changes, attack mitigation, cost control, service provider failures, compliance handling, or other interruptions may occur.',
      },
      {
        text: 'The project owner will strive to maintain stability within a reasonable scope, but does not guarantee that the testing service will be continuous, stable, error-free, bug-free, delay-free, free of data anomalies, or compatible with all devices, browsers, systems, and network environments.',
      },
      {
        text: 'The project owner may suspend, change, discontinue, or terminate all or part of the project at any time for testing objectives, technical conditions, cost pressure, security risks, compliance requirements, personal schedule, project plans, or other reasonable reasons.',
      },
      {
        text: 'After service suspension, discontinuation, or termination, the project owner may stop retaining or delete all or part of the test data, including accounts, characters, progress, items, rankings, mail, storage, logs, and other data.',
      },
      {
        text: 'You understand and agree that service adjustments, interruptions, rollbacks, wipes, stoppage, or termination during testing do not create a compensation obligation, except where mandatory law provides otherwise.',
      },
    ],
  },
  {
    id: 'enforcement',
    article: '18',
    title: 'Enforcement, Account Bans, and Associated Accounts',
    shortTitle: 'Enforcement',
    summary: 'Grants the project owner authority to restrict, freeze, ban, clear data, recover gains, and handle associated accounts.',
    items: [
      {
        text: 'If a player violates laws, these rules, project announcements, game rules, community rules, test rules, or engages in conduct that harms fairness, stability, data validity, community order, or other players, the project owner may take one or more measures based on the nature, impact, intent, abnormal gains, history, associated accounts, and impact on testing order.',
      },
      {
        text: 'Measures may include:',
        children: [
          'Reminder, warning, or request for correction.',
          'Deletion, hiding, collapsing, or restricted display of violating content.',
          'Mute, chat restriction, private message restriction, social restriction, or group restriction.',
          'Trading restriction, mail restriction, ranking restriction, or event participation restriction.',
          'Login restriction, registration restriction, device restriction, IP restriction, or feature restriction.',
          'Recovery of abnormal gains, deletion of abnormal items, or deduction of abnormal currency.',
          'Level rollback, character reset, account data reset, or ranking correction.',
          'Event disqualification, cancellation of test rewards, or cancellation of ranking results.',
          'Temporary account freeze, long-term account freeze, or permanent account ban.',
          'Ban or restriction of associated accounts.',
          'Removal from official communities, restriction of feedback channels, or refusal to process malicious appeals.',
          'Reservation of the right to further pursue responsibility for suspected crimes, server attacks, rights infringement, or major damage.',
        ],
      },
      {
        text: 'For serious violations, the project owner may, without prior warning, directly apply permanent account bans, data clearing, recovery of gains, cancellation of rewards, associated-account bans, device restrictions, or IP restrictions. Serious violations include:',
        children: [
          'Use of cheats, scripts, automation tools, packet tools, API tools, memory editors, or similar cheating tools.',
          'Server attacks, API scanning, verification bypass, malicious stress testing, or interference with normal service operation.',
          'Large-scale exploit abuse, spreading exploit methods, selling exploit methods, or organizing others to exploit bugs.',
          'RMT, account trading, virtual item trading, resource trading, boosting, or organizing prohibited transactions.',
          'Multi-account abuse, mass registration, account networks, resource funneling, restriction evasion, or punishment evasion.',
          'Scams, account theft, impersonating administrators, impersonating the project owner, inducing payments, or leaking account information.',
          'Publishing illegal, pornographic, gambling, scam, cheat-advertising, trading-advertising, or seriously disruptive content.',
          'Malicious complaints, forged evidence, threats against the project owner, attacks on support channels, or repeated disruption of testing order.',
        ],
      },
      {
        text: 'If multiple accounts share the same or highly similar device environment, IP, network environment, browser environment, or login credentials, have obvious resource funneling, coordinated farming, restriction evasion, joint participation in a violation, evasion after a main account is sanctioned, or otherwise appear to be under common control based on logs and behavior, the project owner may treat them as associated accounts.',
      },
      {
        text: 'Handling of associated accounts may include login restriction, registration restriction, feature restriction, recovery of gains, data clearing, account bans, device restrictions, IP restrictions, or other necessary measures.',
      },
      {
        text: 'You understand and agree that accounts, characters, virtual items, rankings, event records, and other data during testing are test data. If violations lead to account restrictions, freezes, bans, data clearing, gain recovery, reward cancellation, ranking cancellation, or community removal, the project owner does not assume compensation liability.',
      },
      {
        text: 'The project owner may determine violations based on server logs, login records, device information, IP information, operation records, API records, chat records, resource flows, transaction records, ranking changes, abnormal behavior patterns, player reports, and other reasonable evidence.',
      },
      {
        text: 'To protect risk-control systems, security mechanisms, and other players, the project owner is not required to disclose complete detection rules, risk models, decision details, logs, or information that may be used to evade detection.',
      },
      {
        text: 'Enforcement does not mean that the project owner waives any other rights. If player conduct causes losses to the project owner, other players, or third parties, the project owner reserves the right to pursue further responsibility.',
      },
    ],
  },
  {
    id: 'appeals',
    article: '19',
    title: 'Appeals',
    shortTitle: 'Appeals',
    summary: 'Allows players to appeal bans or account handling while preserving the project owner’s ability to maintain sanctions.',
    items: [
      {
        text: `Players who object to account handling, data handling, bans, abnormal-gain recovery, or other decisions may submit an appeal through ${props.contactEmail} or another designated appeal channel.`,
      },
      {
        text: 'Appeals should include the account, character name, server, handling time, reasons for appeal, relevant screenshots, operation descriptions, and other evidence supporting the claim.',
      },
      {
        text: 'The project owner will review appeals within a reasonable time based on server logs, operation records, system records, feedback records, violation records, and materials submitted by the player.',
      },
      {
        text: 'During an appeal, the project owner may maintain the original enforcement measure.',
      },
      {
        text: 'If review confirms that the enforcement was correct, the project owner may maintain the original decision. If review confirms a mistake, the project owner may restore the account, restore partial data, or take other reasonable measures at its discretion.',
      },
      {
        text: 'Because the project is in testing and resources are limited, the project owner does not promise immediate processing of all appeals or restoration of all data.',
      },
      {
        text: 'For clearly malicious appeals, repeated harassment, forged evidence, insults, threats, attacks on support, or disruption of normal handling, the project owner may refuse to process the appeal or impose further restrictions.',
      },
    ],
  },
  {
    id: 'disclaimer',
    article: '20',
    title: 'Disclaimer and Limitation of Liability',
    shortTitle: 'Disclaimer',
    summary: 'Limits liability within the scope permitted by law while preserving non-waivable rights.',
    items: [
      {
        text: `To the extent permitted by law, the project owner makes no guarantee that the testing version of ${props.projectName} will be continuously available, stable, accurate, complete, secure, compatible, error-free, bug-free, uninterrupted, free from data loss, or suitable for your personal expectations.`,
      },
      {
        text: 'The project owner may handle issues caused by the inherent instability of testing, including lag, disconnections, login failures, data anomalies, reward anomalies, ranking changes, balance adjustments, feature closures, maintenance, rollbacks, wipes, service stoppage, or termination, within a reasonable scope, but does not promise compensation.',
      },
      {
        text: 'Losses caused by your own conduct, including account sharing, password disclosure, scams, private transactions, RMT, account trading, resource trading, unofficial links, unofficial clients, cheats, rule violations, device issues, network issues, misoperation, or voluntary disclosure of personal information, shall be borne by you.',
      },
      {
        text: 'For losses caused by third-party services, network environments, browsers, operating systems, cloud services, email services, security services, community platforms, network operators, hacking, DDoS attacks, natural disasters, government actions, changes in laws or regulations, force majeure, or other causes not fully controllable by the project owner, the project owner may assist within a reasonable scope but does not assume liability beyond what is legally required.',
      },
      {
        text: `${props.projectName} is currently a testing version for personal learning and project showcase purposes. The project owner does not promise the same level of support, maintenance, stability, data protection, long-term updates, or service continuity as a commercial final-release game.`,
      },
      {
        text: 'The disclaimers and limitations of liability in these rules do not exclude or limit mandatory legal rights, and do not exempt the project owner from liabilities that cannot be legally exempted, including liability arising from intentional misconduct, gross negligence, unlawful processing of personal information, or other non-waivable obligations.',
      },
      {
        text: 'If any provision of these rules is found invalid, partially invalid, or unenforceable, the validity of the remaining provisions shall not be affected. The invalid or unenforceable part shall be interpreted, adjusted, or replaced in a manner closest to the original purpose and consistent with applicable law.',
      },
    ],
  },
  {
    id: 'updates',
    article: '21',
    title: 'Rule Updates',
    shortTitle: 'Updates',
    summary: 'Allows updates to these rules and explains continued use after updates.',
    items: [
      {
        text: 'The project owner may update these rules based on testing progress, version adjustments, technical changes, server conditions, risk control, compliance requirements, community feedback, or other reasonable reasons.',
      },
      {
        text: 'Updated rules may be published through the game page, login page, announcement page, community, pop-up, email, or other reasonable methods.',
      },
      {
        text: 'If you do not agree to updated rules, you should immediately stop accessing, logging in to, or using the project.',
      },
      {
        text: 'Your continued access, login, or use after rule updates means that you have read, understood, and accepted the updated rules.',
      },
    ],
  },
  {
    id: 'law',
    article: '22',
    title: 'Applicable Law and Dispute Resolution',
    shortTitle: 'Law',
    summary: 'Provides a basic dispute resolution clause.',
    items: [
      {
        text: `The formation, validity, performance, interpretation, and dispute resolution of these rules shall be governed by ${props.applicableLaw}.`,
      },
      {
        text: `Any dispute arising from these rules or the testing service of ${props.projectName} should first be resolved through friendly negotiation.`,
      },
      {
        text: `Players may contact the project owner at ${props.contactEmail} to submit questions, feedback, or appeals.`,
      },
      {
        text: 'If negotiation fails, either party may submit the dispute to a court or dispute resolution body with competent jurisdiction according to applicable law.',
      },
    ],
  },
  {
    id: 'confirmation',
    article: '23',
    title: 'Player Confirmation',
    shortTitle: 'Confirmation',
    summary: 'Summarizes the user’s acknowledgements.',
    items: [
      {
        text: `You confirm that you understand ${props.projectName} is currently an early testing version for personal learning, technical validation, and project showcase purposes.`,
      },
      {
        text: 'You understand that the current testing version is not a final release version.',
      },
      {
        text: 'You understand that testing may involve lag, disconnections, errors, bugs, data anomalies, rollbacks, wipes, maintenance, interruptions, shutdowns, or service termination.',
      },
      {
        text: 'You understand that test data has no real-world monetary value and is not guaranteed to be permanently retained.',
      },
      {
        text: 'You understand that the current testing stage does not provide in-game top-up, in-app purchases, or in-game commercial paid services.',
      },
      {
        text: 'You promise not to use cheats, scripts, exploits, private transactions, RMT, multi-account abuse, mass accounts, account networks, boosting, server attacks, or other methods that disrupt testing order.',
      },
      {
        text: 'You understand and accept that the project owner may restrict, freeze, ban, clear data, recover gains, or cancel rewards based on violations, abnormal behavior, account associations, or risk-control judgments.',
      },
      {
        text: 'You understand and accept the testing risks, limitations of liability, data handling, account handling, enforcement measures, and disclaimers in these rules.',
      },
    ],
  },
]

const acknowledgements: Acknowledgement[] = [
  {
    id: 'testing',
    text: `I understand that ${props.projectName} is an early testing version for personal learning, technical validation, and project showcase purposes, and is not a final release version.`,
  },
  {
    id: 'data',
    text: 'I understand that test data has no real-world monetary value, may be adjusted, rolled back, wiped, or deleted, and is not guaranteed to be permanently retained.',
  },
  {
    id: 'rules',
    text: 'I agree not to use cheats, scripts, exploits, RMT, account trading, prohibited player trading, multi-account abuse, account networks, boosting, or other conduct that disrupts testing order.',
  },
  {
    id: 'enforcement',
    text: 'I understand that violations may result in account restrictions, freezing, permanent bans, data clearing, gain recovery, cancellation of rewards, and handling of associated accounts.',
  },
]

const canAccept = computed(() => checked.value.length === acknowledgements.length)

function accept() {
  if (!canAccept.value) return
  emit('accept')
}
</script>
