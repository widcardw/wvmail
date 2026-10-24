import { createRouter, createWebHashHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import MailView from '../views/MailView.vue'
import ReminderView from '../views/ReminderView.vue'
import NotesView from '../views/NotesView.vue'
import SettingsMain from '../views/settings/SettingsMain.vue'
import SettingsCommon from '../views/settings/Common.vue'
import SettingsSecurity from '../views/settings/Security.vue'
import SettingsProxy from '../views/settings/Proxy.vue'
import SettingsAccount from '../views/settings/Account.vue'
import SettingsAbout from '../views/settings/About.vue'
import SettingsLLM from '../views/settings/LLM.vue'
import SettingsTranslation from '../views/settings/Translation.vue'

// NOTE: Headless-migration — shell '/', Notes, Reminder and Settings family are live.
// Mail (MailView/EmailList/EmailDetail) re-enters after conversion.
const routes: RouteRecordRaw[] = [
  // Main mailbox view now that EmailList/EmailDetail are naive-free
  { path: '/', name: 'home', component: MailView },
  { path: '/reminder', name: 'reminder', component: ReminderView },
  { path: '/notes', name: 'notes', component: NotesView },
  {
    path: '/settings',
    name: 'settings',
    component: SettingsMain,
    redirect: { name: 'settings-common' },
    children: [
      { path: 'common', name: 'settings-common', component: SettingsCommon },
      { path: 'accounts', name: 'settings-accounts', component: SettingsAccount },
      { path: 'security', name: 'settings-security', component: SettingsSecurity },
      { path: 'proxy', name: 'settings-proxy', component: SettingsProxy },
      { path: 'llm', name: 'settings-llm', component: SettingsLLM },
      { path: 'translation', name: 'settings-translation', component: SettingsTranslation },
      { path: 'about', name: 'settings-about', component: SettingsAbout },
    ],
  },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

export default router
