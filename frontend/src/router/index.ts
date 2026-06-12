import { createRouter, createWebHashHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import MailView from '../views/MailView.vue'
import NotesView from '../views/NotesView.vue'
import ReminderView from '../views/ReminderView.vue'
import SettingsMain from '../views/settings/SettingsMain.vue'
import SettingsCommon from '../views/settings/Common.vue'
import SettingsAccount from '../views/settings/Account.vue'
import SettingsAbout from '../views/settings/About.vue'
import SettingsLLM from '../views/settings/LLM.vue'
import SettingsTranslation from '../views/settings/Translation.vue'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'mail',
    component: MailView,
  },
  {
    path: '/notes',
    name: 'notes',
    component: NotesView,
  },
  {
    path: '/reminder',
    name: 'reminder',
    component: ReminderView,
  },
  {
    path: '/settings',
    name: 'settings',
    component: SettingsMain,
    redirect: { name: 'settings-common' },
    children: [
      {
        path: '/settings/common',
        name: 'settings-common',
        component: SettingsCommon,
      },
      {
        path: '/settings/accounts',
        name: 'settings-accounts',
        component: SettingsAccount,
      },
      {
        path: '/settings/about',
        name: 'settings-about',
        component: SettingsAbout,
      },
      {
        path: '/settings/llm',
        name: 'settings-llm',
        component: SettingsLLM,
      },
      {
        path: '/settings/translation',
        name: 'settings-translation',
        component: SettingsTranslation,
      },
    ],
  },
]

const router = createRouter({
  history: createWebHashHistory(),
  routes,
})

export default router
