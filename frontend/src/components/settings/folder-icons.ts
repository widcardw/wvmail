/**
 * Folder-icon catalogue shared by the folder-settings picker and `uno.config.ts`.
 *
 * UnoCSS only emits the icon classes it can find in source, while the picker
 * renders `i-${value}` at runtime. uno.config.ts therefore imports
 * FOLDER_ICON_VALUES from here and safelists it, so the picker and the
 * generated CSS can never drift apart.
 *
 * `labelKey` is type-checked against TranslationSchema, so every icon keeps a
 * localized label in both locales.
 */
import type { TranslationSchema } from '../../i18n'

export interface FolderIconChoice {
  value: string
  labelKey: keyof TranslationSchema
}

export interface FolderIconGroup {
  labelKey: keyof TranslationSchema
  icons: FolderIconChoice[]
}

/** The empty value means "keep the automatic icon". */
export const AUTO_FOLDER_ICON: FolderIconChoice = {
  value: '',
  labelKey: 'settings.accounts.folder_icons.auto',
}

export const FOLDER_ICON_GROUPS: FolderIconGroup[] = [
  {
    labelKey: 'settings.accounts.folder_icons.group_mail',
    icons: [
      { value: 'ri:inbox-2-line', labelKey: 'settings.accounts.folder_icons.inbox' },
      { value: 'ri:inbox-archive-line', labelKey: 'settings.accounts.folder_icons.inboxArchive' },
      { value: 'ri:mail-line', labelKey: 'settings.accounts.folder_icons.mail' },
      { value: 'ri:mail-open-line', labelKey: 'settings.accounts.folder_icons.mailOpen' },
      { value: 'ri:mail-unread-line', labelKey: 'settings.accounts.folder_icons.mailUnread' },
      { value: 'ri:send-plane-line', labelKey: 'settings.accounts.folder_icons.sent' },
      { value: 'ri:draft-line', labelKey: 'settings.accounts.folder_icons.drafts' },
      { value: 'ri:archive-2-line', labelKey: 'settings.accounts.folder_icons.archive' },
      { value: 'ri:spam-2-line', labelKey: 'settings.accounts.folder_icons.spam' },
      { value: 'ri:delete-bin-5-line', labelKey: 'settings.accounts.folder_icons.trash' },
      { value: 'ri:folder-2-line', labelKey: 'settings.accounts.folder_icons.folder' },
      { value: 'ri:folder-open-line', labelKey: 'settings.accounts.folder_icons.folderOpen' },
      { value: 'ri:folder-shared-line', labelKey: 'settings.accounts.folder_icons.folderShared' },
      { value: 'ri:file-list-3-line', labelKey: 'settings.accounts.folder_icons.fileList' },
    ],
  },
  {
    labelKey: 'settings.accounts.folder_icons.group_marks',
    icons: [
      { value: 'ri:star-line', labelKey: 'settings.accounts.folder_icons.starred' },
      { value: 'ri:star-half-line', labelKey: 'settings.accounts.folder_icons.starHalf' },
      { value: 'ri:bookmark-3-line', labelKey: 'settings.accounts.folder_icons.bookmark' },
      { value: 'ri:flag-line', labelKey: 'settings.accounts.folder_icons.flag' },
      { value: 'ri:flag-2-line', labelKey: 'settings.accounts.folder_icons.flagAlt' },
      { value: 'ri:notification-3-line', labelKey: 'settings.accounts.folder_icons.important' },
      { value: 'ri:pushpin-line', labelKey: 'settings.accounts.folder_icons.pinned' },
    ],
  },
  {
    labelKey: 'settings.accounts.folder_icons.group_people',
    icons: [
      { value: 'ri:user-3-line', labelKey: 'settings.accounts.folder_icons.person' },
      { value: 'ri:user-line', labelKey: 'settings.accounts.folder_icons.user' },
      { value: 'ri:user-star-line', labelKey: 'settings.accounts.folder_icons.vip' },
      { value: 'ri:group-line', labelKey: 'settings.accounts.folder_icons.group' },
      { value: 'ri:team-line', labelKey: 'settings.accounts.folder_icons.people' },
      { value: 'ri:contacts-book-line', labelKey: 'settings.accounts.folder_icons.addressBook' },
      { value: 'ri:customer-service-2-line', labelKey: 'settings.accounts.folder_icons.service' },
      { value: 'ri:chat-3-line', labelKey: 'settings.accounts.folder_icons.chat' },
      { value: 'ri:message-3-line', labelKey: 'settings.accounts.folder_icons.message' },
    ],
  },
  {
    labelKey: 'settings.accounts.folder_icons.group_work',
    icons: [
      { value: 'ri:briefcase-line', labelKey: 'settings.accounts.folder_icons.work' },
      { value: 'ri:briefcase-2-line', labelKey: 'settings.accounts.folder_icons.project' },
      { value: 'ri:building-4-line', labelKey: 'settings.accounts.folder_icons.company' },
      { value: 'ri:bank-line', labelKey: 'settings.accounts.folder_icons.bank' },
      { value: 'ri:store-line', labelKey: 'settings.accounts.folder_icons.store' },
      { value: 'ri:shopping-bag-3-line', labelKey: 'settings.accounts.folder_icons.shopping' },
      { value: 'ri:shopping-cart-2-line', labelKey: 'settings.accounts.folder_icons.purchase' },
      { value: 'ri:coupon-3-line', labelKey: 'settings.accounts.folder_icons.coupon' },
      { value: 'ri:bill-line', labelKey: 'settings.accounts.folder_icons.bill' },
      { value: 'ri:receipt-line', labelKey: 'settings.accounts.folder_icons.receipt' },
      { value: 'ri:wallet-3-line', labelKey: 'settings.accounts.folder_icons.wallet' },
      { value: 'ri:price-tag-3-line', labelKey: 'settings.accounts.folder_icons.tag' },
      { value: 'ri:refund-2-line', labelKey: 'settings.accounts.folder_icons.refund' },
      { value: 'ri:percent-line', labelKey: 'settings.accounts.folder_icons.discount' },
      { value: 'ri:money-dollar-circle-line', labelKey: 'settings.accounts.folder_icons.payment' },
      { value: 'ri:safe-line', labelKey: 'settings.accounts.folder_icons.safe' },
    ],
  },
  {
    labelKey: 'settings.accounts.folder_icons.group_travel',
    icons: [
      { value: 'ri:flight-takeoff-line', labelKey: 'settings.accounts.folder_icons.travel' },
      { value: 'ri:plane-line', labelKey: 'settings.accounts.folder_icons.flight' },
      { value: 'ri:train-line', labelKey: 'settings.accounts.folder_icons.train' },
      { value: 'ri:car-line', labelKey: 'settings.accounts.folder_icons.car' },
      { value: 'ri:bus-line', labelKey: 'settings.accounts.folder_icons.bus' },
      { value: 'ri:ship-line', labelKey: 'settings.accounts.folder_icons.ship' },
      { value: 'ri:map-2-line', labelKey: 'settings.accounts.folder_icons.map' },
      { value: 'ri:map-pin-line', labelKey: 'settings.accounts.folder_icons.location' },
      { value: 'ri:road-map-line', labelKey: 'settings.accounts.folder_icons.route' },
      { value: 'ri:hotel-line', labelKey: 'settings.accounts.folder_icons.hotel' },
      { value: 'ri:suitcase-line', labelKey: 'settings.accounts.folder_icons.luggage' },
    ],
  },
  {
    labelKey: 'settings.accounts.folder_icons.group_life',
    icons: [
      { value: 'ri:home-2-line', labelKey: 'settings.accounts.folder_icons.home' },
      { value: 'ri:plant-line', labelKey: 'settings.accounts.folder_icons.plants' },
      { value: 'ri:cup-line', labelKey: 'settings.accounts.folder_icons.drinks' },
      { value: 'ri:restaurant-2-line', labelKey: 'settings.accounts.folder_icons.dining' },
      { value: 'ri:cake-3-line', labelKey: 'settings.accounts.folder_icons.birthday' },
      { value: 'ri:basketball-line', labelKey: 'settings.accounts.folder_icons.sports' },
      { value: 'ri:run-line', labelKey: 'settings.accounts.folder_icons.fitness' },
      { value: 'ri:heart-3-line', labelKey: 'settings.accounts.folder_icons.health' },
      { value: 'ri:heart-pulse-line', labelKey: 'settings.accounts.folder_icons.medical' },
      { value: 'ri:capsule-line', labelKey: 'settings.accounts.folder_icons.medicine' },
      { value: 'ri:sun-line', labelKey: 'settings.accounts.folder_icons.daily' },
      { value: 'ri:moon-clear-line', labelKey: 'settings.accounts.folder_icons.night' },
      { value: 'ri:gift-line', labelKey: 'settings.accounts.folder_icons.gift' },
      { value: 'ri:ticket-2-line', labelKey: 'settings.accounts.folder_icons.tickets' },
      { value: 'ri:lightbulb-line', labelKey: 'settings.accounts.folder_icons.ideas' },
      { value: 'ri:seedling-line', labelKey: 'settings.accounts.folder_icons.growth' },
      { value: 'ri:leaf-line', labelKey: 'settings.accounts.folder_icons.eco' },
      { value: 'ri:recycle-line', labelKey: 'settings.accounts.folder_icons.recycle' },
      { value: 'ri:calendar-event-line', labelKey: 'settings.accounts.folder_icons.calendar' },
      { value: 'ri:time-line', labelKey: 'settings.accounts.folder_icons.time' },
    ],
  },
  {
    labelKey: 'settings.accounts.folder_icons.group_tech',
    icons: [
      { value: 'ri:code-s-slash-line', labelKey: 'settings.accounts.folder_icons.code' },
      { value: 'ri:terminal-line', labelKey: 'settings.accounts.folder_icons.terminal' },
      { value: 'ri:cpu-line', labelKey: 'settings.accounts.folder_icons.hardware' },
      { value: 'ri:database-2-line', labelKey: 'settings.accounts.folder_icons.database' },
      { value: 'ri:server-line', labelKey: 'settings.accounts.folder_icons.server' },
      { value: 'ri:cloud-line', labelKey: 'settings.accounts.folder_icons.cloud' },
      { value: 'ri:wifi-line', labelKey: 'settings.accounts.folder_icons.network' },
      { value: 'ri:computer-line', labelKey: 'settings.accounts.folder_icons.computer' },
      { value: 'ri:smartphone-line', labelKey: 'settings.accounts.folder_icons.mobile' },
      { value: 'ri:keyboard-line', labelKey: 'settings.accounts.folder_icons.keyboard' },
      { value: 'ri:robot-2-line', labelKey: 'settings.accounts.folder_icons.robot' },
      { value: 'ri:settings-3-line', labelKey: 'settings.accounts.folder_icons.system' },
      { value: 'ri:tools-line', labelKey: 'settings.accounts.folder_icons.tools' },
      { value: 'ri:hammer-line', labelKey: 'settings.accounts.folder_icons.maintenance' },
      { value: 'ri:palette-line', labelKey: 'settings.accounts.folder_icons.design' },
      { value: 'ri:gamepad-line', labelKey: 'settings.accounts.folder_icons.games' },
      { value: 'ri:puzzle-line', labelKey: 'settings.accounts.folder_icons.plugins' },
      { value: 'ri:github-line', labelKey: 'settings.accounts.folder_icons.repo' },
    ],
  },
  {
    labelKey: 'settings.accounts.folder_icons.group_media',
    icons: [
      { value: 'ri:file-text-line', labelKey: 'settings.accounts.folder_icons.file' },
      { value: 'ri:file-line', labelKey: 'settings.accounts.folder_icons.filePlain' },
      { value: 'ri:newspaper-line', labelKey: 'settings.accounts.folder_icons.news' },
      { value: 'ri:article-line', labelKey: 'settings.accounts.folder_icons.article' },
      { value: 'ri:book-open-line', labelKey: 'settings.accounts.folder_icons.reading' },
      { value: 'ri:image-line', labelKey: 'settings.accounts.folder_icons.image' },
      { value: 'ri:camera-line', labelKey: 'settings.accounts.folder_icons.photo' },
      { value: 'ri:video-line', labelKey: 'settings.accounts.folder_icons.video' },
      { value: 'ri:film-line', labelKey: 'settings.accounts.folder_icons.film' },
      { value: 'ri:music-2-line', labelKey: 'settings.accounts.folder_icons.music' },
      { value: 'ri:mic-line', labelKey: 'settings.accounts.folder_icons.podcast' },
      { value: 'ri:clipboard-line', labelKey: 'settings.accounts.folder_icons.clipboard' },
      { value: 'ri:task-line', labelKey: 'settings.accounts.folder_icons.tasks' },
    ],
  },
  {
    labelKey: 'settings.accounts.folder_icons.group_misc',
    icons: [
      { value: 'ri:shield-keyhole-line', labelKey: 'settings.accounts.folder_icons.security' },
      { value: 'ri:shield-check-line', labelKey: 'settings.accounts.folder_icons.verified' },
      { value: 'ri:lock-password-line', labelKey: 'settings.accounts.folder_icons.password' },
      { value: 'ri:key-2-line', labelKey: 'settings.accounts.folder_icons.key' },
      { value: 'ri:fingerprint-line', labelKey: 'settings.accounts.folder_icons.identity' },
      { value: 'ri:megaphone-line', labelKey: 'settings.accounts.folder_icons.announcement' },
      { value: 'ri:advertisement-line', labelKey: 'settings.accounts.folder_icons.promotion' },
      { value: 'ri:award-line', labelKey: 'settings.accounts.folder_icons.award' },
      { value: 'ri:trophy-line', labelKey: 'settings.accounts.folder_icons.achievement' },
      { value: 'ri:earth-line', labelKey: 'settings.accounts.folder_icons.global' },
      { value: 'ri:fire-line', labelKey: 'settings.accounts.folder_icons.hot' },
      { value: 'ri:rocket-line', labelKey: 'settings.accounts.folder_icons.launch' },
      { value: 'ri:error-warning-line', labelKey: 'settings.accounts.folder_icons.warning' },
      { value: 'ri:information-line', labelKey: 'settings.accounts.folder_icons.info' },
    ],
  },
]

export const FOLDER_ICON_CHOICES: FolderIconChoice[] = [
  AUTO_FOLDER_ICON,
  ...FOLDER_ICON_GROUPS.flatMap((group) => group.icons),
]

/** Values that reach the DOM as `i-${value}` classes. */
export const FOLDER_ICON_VALUES: string[] = FOLDER_ICON_CHOICES.map(
  (icon) => icon.value,
).filter(Boolean)
