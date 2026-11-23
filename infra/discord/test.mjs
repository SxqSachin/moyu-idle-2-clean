#!/usr/bin/env node
/**
 * setup-discord-community.mjs
 *
 * 一键搭建一个适合独立网页游戏的中英双语 Discord 社区骨架。
 * It creates a bilingual Discord community structure for an indie browser game.
 *
 * ✅ 可重复运行：已有角色/频道会跳过，不会删除任何内容。
 * ✅ 国内玩家兼容：欢迎页会同时放 QQ 群和统一反馈表入口。
 * ✅ 反馈友好：尝试创建 Forum 频道；如果服务器不支持 Forum，会自动降级为普通文字频道。
 *
 * 使用步骤 / Usage:
 * 1) 安装 Node.js 18+。
 * 2) 在本文件同目录执行：npm init -y && npm i discord.js
 * 3) 创建 .env 文件，填写：
 *
 *    DISCORD_BOT_TOKEN=你的BotToken
 *    DISCORD_GUILD_ID=你的服务器ID
 *    GAME_NAME=你的游戏名
 *    BUG_REPORT_URL=https://你的中英双语反馈表链接
 *    QQ_GROUP=你的QQ群号或加群链接
 *    ITCH_URL=https://your-name.itch.io/your-game
 *    WEBSITE_URL=https://你的游戏官网，可留空
 *
 * 4) 给 Bot 邀请权限：Manage Channels、Manage Roles、View Channels、Send Messages、
 *    Manage Messages、Create Public Threads、Send Messages in Threads、Manage Threads、Read Message History。
 * 5) 确保 Bot 的服务器角色位置高于它要创建/管理的角色。
 * 6) 执行：node setup-discord-community.mjs
 *
 * 安全提醒 / Security:
 * - 不要把真实 DISCORD_BOT_TOKEN 发给别人，也不要提交到公开 GitHub 仓库。
 * - Discord 官方把 Bot Token 视为敏感凭据，泄露后请立刻重置。
 */

import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import {
  ChannelType,
  Client,
  GatewayIntentBits,
  PermissionFlagsBits,
} from 'discord.js';

const ENV_PATH = path.resolve(process.cwd(), '.env');
loadDotEnv(ENV_PATH);

const CONFIG = {
  botToken: env('DISCORD_BOT_TOKEN', env('BOT_TOKEN', '')),
  guildId: env('DISCORD_GUILD_ID', env('GUILD_ID', '')),
  gameName: env('GAME_NAME', 'Your Game'),
  bugReportUrl: env('BUG_REPORT_URL', 'https://example.com/bug-report'),
  qqGroup: env('QQ_GROUP', 'QQ group number / QQ群号'),
  itchUrl: env('ITCH_URL', ''),
  websiteUrl: env('WEBSITE_URL', ''),
  dryRun: ['1', 'true', 'yes'].includes(env('DRY_RUN', '').toLowerCase()),
};

const ROLE_SPECS = [
  { key: 'coreTeam', name: 'Core Team', color: 0xf1c40f, hoist: true },
  { key: 'moderator', name: 'Moderator', color: 0x5865f2, hoist: true },
  { key: 'tester', name: 'Playtester', color: 0x57f287, hoist: false },
  { key: 'translator', name: 'Translator', color: 0x3498db, hoist: false },
  { key: 'bugHunter', name: 'Bug Hunter', color: 0xed4245, hoist: false },
  { key: 'english', name: 'English', color: 0x95a5a6, hoist: false },
  { key: 'chinese', name: '中文', color: 0x95a5a6, hoist: false },
  { key: 'announcementsPing', name: 'Announcements Ping', color: 0x9b59b6, hoist: false },
  { key: 'patchNotesPing', name: 'Patch Notes Ping', color: 0x1abc9c, hoist: false },
];

const CATEGORY_SPECS = [
  { key: 'start', name: 'START HERE｜开始' },
  { key: 'news', name: 'NEWS｜公告' },
  { key: 'community', name: 'COMMUNITY｜社区' },
  { key: 'feedback', name: 'FEEDBACK｜反馈' },
  { key: 'languages', name: 'LANGUAGES｜语言' },
  { key: 'staff', name: 'STAFF｜管理组' },
];

const TEXT_CHANNEL_SPECS = [
  {
    category: 'start',
    name: 'welcome',
    topic: 'Start here. Read this first.｜新玩家请先看这里。',
    message: welcomeMessage,
  },
  {
    category: 'start',
    name: 'rules',
    topic: 'Community rules.｜社区规则。',
    message: rulesMessage,
  },
  {
    category: 'start',
    name: 'faq',
    topic: 'Frequently asked questions.｜常见问题。',
    message: faqMessage,
  },
  {
    category: 'start',
    name: 'how-to-report-bugs',
    topic: 'Bug report guide and template.｜问题反馈指南和模板。',
    message: bugGuideMessage,
  },
  {
    category: 'news',
    name: 'announcements',
    topic: 'Official announcements only.｜官方公告频道。',
    readOnly: true,
    message: announcementsMessage,
  },
  {
    category: 'news',
    name: 'patch-notes',
    topic: 'Version updates and fixes.｜版本更新和修复记录。',
    readOnly: true,
    message: patchNotesMessage,
  },
  {
    category: 'community',
    name: 'general',
    topic: 'General discussion.｜闲聊与普通讨论。',
    slowmodeSeconds: 3,
    message: generalMessage,
  },
  {
    category: 'community',
    name: 'screenshots',
    topic: 'Share screenshots, clips, records, and discoveries.｜分享截图、录像、记录和发现。',
    slowmodeSeconds: 5,
    message: screenshotsMessage,
  },
  {
    category: 'community',
    name: 'help',
    topic: 'Ask for help from players and moderators.｜玩家互助与轻量求助。',
    slowmodeSeconds: 5,
    message: helpMessage,
  },
  {
    category: 'languages',
    name: 'english',
    topic: 'English chat.｜英文交流区。',
    slowmodeSeconds: 3,
    message: englishMessage,
  },
  {
    category: 'languages',
    name: 'chinese',
    topic: '中文交流区。Chinese chat.',
    slowmodeSeconds: 3,
    message: chineseMessage,
  },
  {
    category: 'staff',
    name: 'staff-chat',
    topic: 'Private staff discussion.｜管理组内部讨论。',
    staffOnly: true,
    message: staffChatMessage,
  },
  {
    category: 'staff',
    name: 'triage',
    topic: 'Private bug triage and moderation notes.｜内部问题整理和处理记录。',
    staffOnly: true,
    message: triageMessage,
  },
];

const FORUM_CHANNEL_SPECS = [
  {
    category: 'feedback',
    name: 'bug-reports',
    topic: 'One bug per post. Use tags and include steps to reproduce.｜一个问题一个帖子，请写复现步骤。',
    tags: [
      'Need Info', 'Confirmed', 'In Progress', 'Fixed', 'Duplicate',
      'Critical', 'Login', 'Gameplay', 'UI', 'Performance', 'Mobile', 'Desktop', 'Translation',
    ],
    starterTitle: 'Bug report template / 问题反馈模板',
    starterMessage: bugForumStarterMessage,
  },
  {
    category: 'feedback',
    name: 'suggestions',
    topic: 'Feature requests, balance ideas, and long-term improvements.｜功能建议、平衡建议和长期改进。',
    tags: ['Idea', 'Balance', 'Quality of Life', 'Planned', 'In Progress', 'Done', 'Not Planned'],
    starterTitle: 'Suggestion template / 建议反馈模板',
    starterMessage: suggestionForumStarterMessage,
  },
  {
    category: 'feedback',
    name: 'translation-feedback',
    topic: 'Localization issues, wording, typos, and translation suggestions.｜翻译、错别字和本地化建议。',
    tags: ['English', '简体中文', 'Typo', 'Wording', 'Missing Text', 'Fixed'],
    starterTitle: 'Translation feedback template / 翻译反馈模板',
    starterMessage: translationForumStarterMessage,
  },
];

function env(name, fallback = '') {
  const value = process.env[name];
  return value === undefined || value === null || value === '' ? fallback : value;
}

function loadDotEnv(filePath) {
  if (!fs.existsSync(filePath)) return;
  const text = fs.readFileSync(filePath, 'utf8');
  for (const rawLine of text.split(/\r?\n/)) {
    const line = rawLine.trim();
    if (!line || line.startsWith('#')) continue;
    const match = line.match(/^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.*)$/);
    if (!match) continue;
    const [, key, rawValue] = match;
    if (process.env[key] !== undefined) continue;
    let value = rawValue.trim();
    if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
      value = value.slice(1, -1);
    }
    process.env[key] = value;
  }
}

function requireConfig() {
  const missing = [];
  if (!CONFIG.botToken) missing.push('DISCORD_BOT_TOKEN');
  if (!CONFIG.guildId) missing.push('DISCORD_GUILD_ID');
  if (missing.length > 0) {
    console.error(`\nMissing required config: ${missing.join(', ')}`);
    console.error('Create a .env file next to this script, or set environment variables before running.\n');
    process.exit(1);
  }
}

function printDryRunPlan() {
  console.log('\nDRY_RUN=1 is enabled. No Discord API calls will be made.\n');
  console.log(`Game: ${CONFIG.gameName}`);
  console.log('\nRoles:');
  for (const role of ROLE_SPECS) console.log(`- ${role.name}`);
  console.log('\nCategories and channels:');
  for (const category of CATEGORY_SPECS) {
    console.log(`\n[${category.name}]`);
    for (const channel of TEXT_CHANNEL_SPECS.filter((item) => item.category === category.key)) {
      console.log(`- #${channel.name}`);
    }
    for (const channel of FORUM_CHANNEL_SPECS.filter((item) => item.category === category.key)) {
      console.log(`- forum: ${channel.name}`);
    }
  }
  console.log('\nRun without DRY_RUN=1 to create everything.\n');
}

async function main() {
  if (CONFIG.dryRun) {
    printDryRunPlan();
    return;
  }

  requireConfig();

  const client = new Client({ intents: [GatewayIntentBits.Guilds] });

  client.once('ready', async () => {
    try {
      console.log(`Logged in as ${client.user.tag}`);
      const guild = await client.guilds.fetch(CONFIG.guildId);
      await guild.channels.fetch();
      await guild.roles.fetch();
      const botMember = await guild.members.fetch(client.user.id);

      console.log(`Setting up server: ${guild.name}`);

      const roles = await ensureRoles(guild);
      const categories = await ensureCategories(guild);

      for (const spec of TEXT_CHANNEL_SPECS) {
        const parent = categories[spec.category];
        const result = await ensureTextChannel(guild, parent, spec, roles, botMember);
        if (result.created && spec.message) {
          await sendAndTryPin(result.channel, spec.message());
        }
      }

      for (const spec of FORUM_CHANNEL_SPECS) {
        const parent = categories[spec.category];
        await ensureForumOrFallbackTextChannel(guild, parent, spec, roles, botMember);
      }

      console.log('\nDone. Your Discord community skeleton has been created.');
      console.log('Next steps:');
      console.log('1) Manually review permissions, especially staff-only channels.');
      console.log('2) Add real links for bug report form, QQ group, itch.io, and website if you used placeholders.');
      console.log('3) Create an invite link and add it to your game, itch.io page, and announcements.\n');
    } catch (error) {
      console.error('\nSetup failed:');
      console.error(error);
      console.error('\nCommon fixes:');
      console.error('- Make sure the bot is already invited to the server.');
      console.error('- Give the bot Manage Channels and Manage Roles permissions.');
      console.error('- Move the bot role above the roles it needs to create or edit.');
      console.error('- Check DISCORD_GUILD_ID and DISCORD_BOT_TOKEN.\n');
      process.exitCode = 1;
    } finally {
      client.destroy();
    }
  });

  await client.login(CONFIG.botToken);
}

async function ensureRoles(guild) {
  const roles = {};
  for (const spec of ROLE_SPECS) {
    let role = guild.roles.cache.find((item) => item.name === spec.name);
    if (role) {
      console.log(`Role exists: ${spec.name}`);
    } else {
      role = await guild.roles.create({
        name: spec.name,
        color: spec.color,
        hoist: Boolean(spec.hoist),
        mentionable: false,
        permissions: [],
        reason: 'Initial community setup script',
      });
      console.log(`Created role: ${spec.name}`);
    }
    roles[spec.key] = role;
  }
  return roles;
}

async function ensureCategories(guild) {
  const categories = {};
  for (const spec of CATEGORY_SPECS) {
    let category = guild.channels.cache.find(
      (item) => item.name === spec.name && item.type === ChannelType.GuildCategory,
    );
    if (category) {
      console.log(`Category exists: ${spec.name}`);
    } else {
      category = await guild.channels.create({
        name: spec.name,
        type: ChannelType.GuildCategory,
        reason: 'Initial community setup script',
      });
      console.log(`Created category: ${spec.name}`);
    }
    categories[spec.key] = category;
  }
  return categories;
}

async function ensureTextChannel(guild, parent, spec, roles, botMember) {
  const existing = guild.channels.cache.find(
    (item) => item.name === spec.name && item.parentId === parent.id && item.type === ChannelType.GuildText,
  );
  if (existing) {
    console.log(`Text channel exists: #${spec.name}`);
    return { channel: existing, created: false };
  }

  const channel = await guild.channels.create({
    name: spec.name,
    type: ChannelType.GuildText,
    parent: parent.id,
    topic: withLinks(spec.topic),
    rateLimitPerUser: spec.slowmodeSeconds ?? 0,
    permissionOverwrites: permissionOverwritesFor(guild, roles, botMember, spec),
    reason: 'Initial community setup script',
  });

  console.log(`Created text channel: #${spec.name}`);
  return { channel, created: true };
}

async function ensureForumOrFallbackTextChannel(guild, parent, spec, roles, botMember) {
  const existing = guild.channels.cache.find(
    (item) => item.name === spec.name && item.parentId === parent.id &&
      [ChannelType.GuildForum, ChannelType.GuildText].includes(item.type),
  );
  if (existing) {
    console.log(`Feedback channel exists: ${spec.name}`);
    return { channel: existing, created: false };
  }

  try {
    const forum = await guild.channels.create({
      name: spec.name,
      type: ChannelType.GuildForum,
      parent: parent.id,
      topic: withLinks(spec.topic),
      availableTags: spec.tags.map((name) => ({ name, moderated: false })),
      defaultReactionEmoji: { emoji: '👍' },
      rateLimitPerUser: 10,
      permissionOverwrites: permissionOverwritesFor(guild, roles, botMember, { slowmodeSeconds: 10 }),
      reason: 'Initial community setup script',
    });
    console.log(`Created forum channel: ${spec.name}`);

    try {
      await forum.threads.create({
        name: spec.starterTitle,
        message: { content: spec.starterMessage() },
        reason: 'Initial forum starter post',
      });
      console.log(`Created starter post in forum: ${spec.name}`);
    } catch (error) {
      console.warn(`Could not create starter post in ${spec.name}: ${error.message}`);
    }

    return { channel: forum, created: true };
  } catch (error) {
    console.warn(`Could not create Forum channel ${spec.name}. Falling back to text channel. Reason: ${error.message}`);

    const text = await guild.channels.create({
      name: spec.name,
      type: ChannelType.GuildText,
      parent: parent.id,
      topic: withLinks(spec.topic),
      rateLimitPerUser: 10,
      permissionOverwrites: permissionOverwritesFor(guild, roles, botMember, { slowmodeSeconds: 10 }),
      reason: 'Initial community setup script fallback',
    });
    console.log(`Created fallback text channel: #${spec.name}`);
    await sendAndTryPin(text, `${spec.starterMessage()}\n\n${fallbackNoticeMessage()}`);
    return { channel: text, created: true };
  }
}

function permissionOverwritesFor(guild, roles, botMember, spec) {
  if (spec.staffOnly) return staffOnlyOverwrites(guild, roles, botMember);
  if (spec.readOnly) return readOnlyOverwrites(guild, roles, botMember);
  return publicOverwrites(guild, roles, botMember);
}

function publicOverwrites(guild, roles, botMember) {
  return [
    {
      id: guild.roles.everyone.id,
      allow: [
        PermissionFlagsBits.ViewChannel,
        PermissionFlagsBits.ReadMessageHistory,
        PermissionFlagsBits.SendMessages,
        PermissionFlagsBits.CreatePublicThreads,
        PermissionFlagsBits.SendMessagesInThreads,
        PermissionFlagsBits.AddReactions,
        PermissionFlagsBits.AttachFiles,
        PermissionFlagsBits.EmbedLinks,
      ],
    },
    botAllowOverwrite(botMember),
    staffAllowOverwrite(roles.coreTeam),
    staffAllowOverwrite(roles.moderator),
  ];
}

function readOnlyOverwrites(guild, roles, botMember) {
  return [
    {
      id: guild.roles.everyone.id,
      allow: [PermissionFlagsBits.ViewChannel, PermissionFlagsBits.ReadMessageHistory],
      deny: [
        PermissionFlagsBits.SendMessages,
        PermissionFlagsBits.CreatePublicThreads,
        PermissionFlagsBits.CreatePrivateThreads,
        PermissionFlagsBits.SendMessagesInThreads,
      ],
    },
    botAllowOverwrite(botMember),
    staffAllowOverwrite(roles.coreTeam),
    staffAllowOverwrite(roles.moderator),
  ];
}

function staffOnlyOverwrites(guild, roles, botMember) {
  return [
    {
      id: guild.roles.everyone.id,
      deny: [PermissionFlagsBits.ViewChannel],
    },
    botAllowOverwrite(botMember),
    staffAllowOverwrite(roles.coreTeam),
    staffAllowOverwrite(roles.moderator),
  ];
}

function botAllowOverwrite(botMember) {
  return {
    id: botMember.id,
    allow: [
      PermissionFlagsBits.ViewChannel,
      PermissionFlagsBits.ReadMessageHistory,
      PermissionFlagsBits.SendMessages,
      PermissionFlagsBits.ManageMessages,
      PermissionFlagsBits.ManageChannels,
      PermissionFlagsBits.CreatePublicThreads,
      PermissionFlagsBits.CreatePrivateThreads,
      PermissionFlagsBits.SendMessagesInThreads,
      PermissionFlagsBits.ManageThreads,
      PermissionFlagsBits.AttachFiles,
      PermissionFlagsBits.EmbedLinks,
      PermissionFlagsBits.AddReactions,
    ],
  };
}

function staffAllowOverwrite(role) {
  return {
    id: role.id,
    allow: [
      PermissionFlagsBits.ViewChannel,
      PermissionFlagsBits.ReadMessageHistory,
      PermissionFlagsBits.SendMessages,
      PermissionFlagsBits.ManageMessages,
      PermissionFlagsBits.ManageThreads,
      PermissionFlagsBits.CreatePublicThreads,
      PermissionFlagsBits.CreatePrivateThreads,
      PermissionFlagsBits.SendMessagesInThreads,
      PermissionFlagsBits.AttachFiles,
      PermissionFlagsBits.EmbedLinks,
      PermissionFlagsBits.AddReactions,
    ],
  };
}

async function sendAndTryPin(channel, content) {
  try {
    const message = await channel.send({ content });
    try {
      await message.pin('Initial community guide');
    } catch (error) {
      console.warn(`Could not pin message in #${channel.name}: ${error.message}`);
    }
  } catch (error) {
    console.warn(`Could not send starter message in #${channel.name}: ${error.message}`);
  }
}

function withLinks(text) {
  const links = [];
  if (CONFIG.bugReportUrl) links.push(`Feedback: ${CONFIG.bugReportUrl}`);
  if (CONFIG.itchUrl) links.push(`itch.io: ${CONFIG.itchUrl}`);
  if (CONFIG.websiteUrl) links.push(`Website: ${CONFIG.websiteUrl}`);
  return links.length > 0 ? `${text}\n${links.join(' | ')}` : text;
}

function linksBlock() {
  return [
    CONFIG.itchUrl ? `- itch.io: ${CONFIG.itchUrl}` : null,
    CONFIG.websiteUrl ? `- Website: ${CONFIG.websiteUrl}` : null,
    CONFIG.bugReportUrl ? `- Bug Report / 问题反馈: ${CONFIG.bugReportUrl}` : null,
    CONFIG.qqGroup ? `- QQ Group / QQ群: ${CONFIG.qqGroup}` : null,
  ].filter(Boolean).join('\n');
}

function welcomeMessage() {
  return `# Welcome to ${CONFIG.gameName}! / 欢迎来到 ${CONFIG.gameName}！

Thanks for joining the community. This server is for updates, player discussion, bug reports, suggestions, and playtest coordination.

欢迎加入社区。本服务器用于发布公告、玩家交流、问题反馈、建议收集和测试招募。

## Useful links / 常用链接
${linksBlock()}

## Where should I go? / 我应该去哪里？
- Read rules / 查看规则: #rules
- Check updates / 查看更新: #announcements and #patch-notes
- Chat / 聊天: #general, #english, #chinese
- Need help / 需要帮助: #help
- Report bugs / 报告问题: #bug-reports or the external feedback form

Please do not post passwords, real-name identity information, payment credentials, or private personal information in public channels.\n请不要在公开频道发布密码、实名身份信息、支付凭证或其他隐私信息。`;
}

function rulesMessage() {
  return `# Rules / 社区规则

1. Be respectful. No harassment, hate speech, threats, or personal attacks.\n   尊重他人。禁止骚扰、仇恨言论、威胁和人身攻击。

2. Keep discussion related to the game and community.\n   请尽量围绕游戏和社区交流。

3. No spam, scams, phishing links, or suspicious downloads.\n   禁止刷屏、诈骗、钓鱼链接和可疑下载。

4. Report one bug per post and include steps to reproduce when possible.\n   一个问题请发一个帖子，尽量写清复现步骤。

5. Do not share passwords, tokens, private account data, or other sensitive information.\n   不要分享密码、Token、账号隐私或其他敏感信息。

6. Mods may remove posts or restrict access to keep the community safe.\n   管理员可能会为了社区安全移除内容或限制访问。`;
}

function faqMessage() {
  return `# FAQ / 常见问题

**Q: Where can I play the game? / 在哪里玩游戏？**
${CONFIG.itchUrl ? `A: itch.io page: ${CONFIG.itchUrl}` : 'A: The play link will be added here.'}

**Q: Do I need to download anything? / 需要下载客户端吗？**
A: The game is designed as a browser game. If a download or standalone client becomes available later, we will announce it in #announcements.\nA：这是网页游戏。未来如果有客户端下载版，会在 #announcements 公告。

**Q: Where do I report bugs? / 在哪里反馈问题？**
A: Use #bug-reports or the external bilingual feedback form: ${CONFIG.bugReportUrl}\nA：可以使用 #bug-reports，或填写中英双语反馈表：${CONFIG.bugReportUrl}

**Q: Where should Chinese players chat? / 中文玩家在哪里交流？**
A: You can use #chinese or QQ Group: ${CONFIG.qqGroup}\nA：可以在 #chinese 交流，也可以加入 QQ 群：${CONFIG.qqGroup}`;
}

function bugGuideMessage() {
  return `# How to report bugs / 如何反馈问题

Please use #bug-reports or this external form:\n请使用 #bug-reports，或填写外部反馈表：
${CONFIG.bugReportUrl}

## Good bug report template / 推荐反馈格式
\`\`\`
Issue type / 问题类型:
Account ID or nickname / 账号ID或昵称:
Browser / 浏览器: Chrome / Edge / Firefox / Safari / Other
Device / 设备: PC / Mac / Android / iPhone / iPad / Other
Time / 发生时间:
What happened? / 发生了什么:
What did you expect? / 你原本期望发生什么:
Steps to reproduce / 复现步骤:
Screenshot or video / 截图或录像:
Contact / 联系方式:
\`\`\`

Please do not include your password.\n请不要提交你的密码。`;
}

function announcementsMessage() {
  return `# Announcements / 官方公告

Official news, important notices, events, and playtest announcements will be posted here.\n这里会发布官方消息、重要通知、活动和测试招募。

If you only want updates, you can mute other channels and keep this one enabled.\n如果你只想看更新，可以静音其他频道，只保留本频道通知。`;
}

function patchNotesMessage() {
  return `# Patch Notes / 版本更新

Version updates, bug fixes, balance changes, and known issues will be posted here.\n这里会发布版本更新、Bug 修复、平衡调整和已知问题。

Recommended format:\n建议格式：
\`\`\`
Version / 版本:
Date / 日期:
New / 新增:
Changed / 调整:
Fixed / 修复:
Known issues / 已知问题:
\`\`\``;
}

function generalMessage() {
  return `# General / 闲聊

Talk about the game, share ideas, ask light questions, or meet other players.\n可以聊游戏、分享想法、问轻量问题，也可以认识其他玩家。

For structured bug reports, please use #bug-reports or the feedback form instead.\n严肃问题反馈请使用 #bug-reports 或反馈表。`;
}

function screenshotsMessage() {
  return `# Screenshots / 截图与录像

Share screenshots, short clips, records, funny moments, strategies, and discoveries.\n欢迎分享截图、短视频、记录、趣事、攻略和发现。

Please avoid posting spoilers without a warning.\n涉及剧透请提前说明。`;
}

function helpMessage() {
  return `# Help / 玩家互助

Ask for help here if you are stuck, confused, or unsure whether something is a bug.\n如果你卡关、看不懂系统、或者不确定是不是 Bug，可以先在这里问。

For account-specific or private issues, use the feedback form and avoid posting private data publicly.\n账号相关或隐私问题请走反馈表，不要在公开频道发布敏感信息。`;
}

function englishMessage() {
  return `# English Chat

English discussion channel for players around the world.\nPlease use #bug-reports for structured bug reports and #suggestions for feature ideas.`;
}

function chineseMessage() {
  return `# 中文交流区

中文玩家可以在这里聊天、提问和交流。

国内玩家也可以加入 QQ 群：${CONFIG.qqGroup}

严肃问题反馈建议使用反馈表，方便我们统一整理：${CONFIG.bugReportUrl}`;
}

function staffChatMessage() {
  return `# Staff Chat / 管理组内部频道

Private channel for moderators and team members.\n管理组和开发组内部讨论频道。

Suggested use:\n建议用途：
- Moderation notes / 管理记录
- Community planning / 社区规划
- Event planning / 活动规划
- Escalated player issues / 需要升级处理的问题`;
}

function triageMessage() {
  return `# Triage / 问题整理

Use this channel to summarize, merge, and prioritize bug reports and player feedback.\n用于整理、合并和排优玩家反馈。

Suggested labels:\n建议状态：
- Need Info / 需要更多信息
- Confirmed / 已确认
- In Progress / 处理中
- Fixed / 已修复
- Duplicate / 重复问题
- Won't Fix / 暂不处理`;
}

function bugForumStarterMessage() {
  return `# Bug report template / 问题反馈模板

Please create one post per bug. If the issue contains private account data, use the external feedback form instead.\n请一个 Bug 发一个帖子。如果涉及账号隐私，请使用外部反馈表。

External form / 外部反馈表：${CONFIG.bugReportUrl}

\`\`\`
Issue type / 问题类型:
Account ID or nickname / 账号ID或昵称:
Browser / 浏览器:
Device / 设备:
Time / 发生时间:
What happened? / 发生了什么:
Expected result / 期望结果:
Steps to reproduce / 复现步骤:
Screenshot/video / 截图或录像:
Contact / 联系方式:
\`\`\`

Do not include your password.\n请不要提交你的密码。`;
}

function suggestionForumStarterMessage() {
  return `# Suggestion template / 建议反馈模板

Please describe the problem your idea solves, not just the feature itself.\n请尽量说明这个建议解决什么问题，而不只是提出功能名。

\`\`\`
Suggestion / 建议:
Why it matters / 为什么重要:
Who benefits / 哪类玩家会受益:
Possible downside / 可能的副作用:
Example / 示例:
\`\`\``;
}

function translationForumStarterMessage() {
  return `# Translation feedback template / 翻译反馈模板

Use this for typos, confusing wording, missing text, untranslated text, or localization suggestions.\n这里用于反馈错别字、表达不清、文本缺失、未翻译内容或本地化建议。

\`\`\`
Language / 语言:
Where did you see it? / 出现位置:
Current text / 当前文本:
Suggested text / 建议文本:
Screenshot / 截图:
\`\`\``;
}

function fallbackNoticeMessage() {
  return `Note: This server may not have Forum Channels enabled, so this was created as a normal text channel.\n提示：服务器可能尚未启用 Forum Channels，因此本频道已降级为普通文字频道。`;
}

main().catch((error) => {
  console.error(error);
  process.exit(1);
});