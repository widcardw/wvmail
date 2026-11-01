<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { Menu } from '@ark-ui/vue/menu'
import { Clipboard } from '@wailsio/runtime'
import { useI18n } from '~/i18n'
import { useContactsStore } from '~/stores/contacts'
import { useFeedbackStore } from '~/stores/feedback'
import { useLocaleStore } from '~/stores/locale'
import { formatDate } from '~/composables/formats'
import RecipientList from '~/components/detail/RecipientList.vue'
import type { EmailDetail } from '~/types/email'

// The header of the reading pane: who sent it, when, the subject and the recipients.
// Message-wide actions live in the fixed bar above this scroll area.
// Recipients live in RecipientList: a big-company To/Cc list has to collapse.
const props = defineProps<{
  detail: EmailDetail
}>()

const { t } = useI18n()
const localeStore = useLocaleStore()
const contactsStore = useContactsStore()
const feedback = useFeedbackStore()

interface SenderParts {
  name: string | null
  addr: string | null
  fallback: string | null
}

/** Name+address when both are known, otherwise whatever single field we have. */
function senderDisplay(detail: {
  from_name?: string
  from_addr?: string
  from?: string
}): SenderParts {
  if (detail.from_name && detail.from_addr)
    return { name: detail.from_name, addr: detail.from_addr, fallback: null }
  if (detail.from_name) return { name: null, addr: null, fallback: detail.from_name }
  if (detail.from_addr) return { name: null, addr: null, fallback: detail.from_addr }
  if (detail.from) return { name: null, addr: null, fallback: detail.from }
  return { name: null, addr: null, fallback: 'Unknown' }
}

function extractAddress(raw: string): string {
  const match = raw.match(/<([^>]+)>/)
  return (match?.[1] || raw).trim()
}

const sender = computed(() => {
  const display = senderDisplay(props.detail)
  const raw = props.detail.from_addr || props.detail.from || display.addr || display.fallback || ''
  return {
    name:
      display.name || (display.fallback && !display.fallback.includes('@') ? display.fallback : ''),
    email: extractAddress(raw),
  }
})

const senderIsContact = computed(
  () => Boolean(sender.value.email) && contactsStore.hasEmail(sender.value.email),
)

async function copySenderEmail() {
  if (!sender.value.email) return
  try {
    await Clipboard.SetText(sender.value.email)
    feedback.success(t.value['contacts.copied'])
  } catch (error) {
    console.error('Failed to copy sender email', error)
    feedback.error(t.value['contacts.copyFailed'])
  }
}

async function addSenderContact() {
  if (!sender.value.email || senderIsContact.value) return
  const saved = await contactsStore.addContact(sender.value.name, sender.value.email)
  if (!saved) {
    feedback.error(t.value['contacts.saveFailed'])
    return
  }
  feedback.success(t.value['contacts.added'])
}

async function onSenderAction(details: unknown) {
  const value = (details as { value?: string }).value
  if (value === 'copy-email') await copySenderEmail()
  if (value === 'add-contact') await addSenderContact()
}

onMounted(() => {
  void contactsStore.loadContacts()
})
</script>

<template>
  <div class="detail-header">
    <div class="sender">
      <Menu.Root
        v-if="sender.email"
        :positioning="{ placement: 'bottom-start', gutter: 6 }"
        lazy-mount
        unmount-on-exit
        @select="onSenderAction"
      >
        <Menu.Trigger class="sender-trigger" :aria-label="sender.email">
          <span v-if="senderDisplay(detail).name" class="from">
            <strong>{{ senderDisplay(detail).name }}</strong>
            <span class="addr">&lt;{{ senderDisplay(detail).addr }}&gt;</span>
          </span>
          <span v-else class="from">{{ senderDisplay(detail).fallback }}</span>
          <i class="i-ri:arrow-down-s-line sender-chevron" aria-hidden="true" />
        </Menu.Trigger>
        <Teleport to="body">
          <Menu.Positioner class="sender-menu-pos">
            <Menu.Content class="sender-menu">
              <Menu.Item value="copy-email" class="sender-menu__item">
                <i class="i-ri:file-copy-line" aria-hidden="true" />
                <Menu.ItemText>{{ t['contacts.copyEmail'] }}</Menu.ItemText>
              </Menu.Item>
              <Menu.Item value="add-contact" class="sender-menu__item" :disabled="senderIsContact">
                <i
                  :class="senderIsContact ? 'i-ri:check-line' : 'i-ri:user-add-line'"
                  aria-hidden="true"
                />
                <Menu.ItemText>
                  {{ senderIsContact ? t['contacts.alreadySaved'] : t['contacts.addSender'] }}
                </Menu.ItemText>
              </Menu.Item>
            </Menu.Content>
          </Menu.Positioner>
        </Teleport>
      </Menu.Root>
      <span v-else class="from">{{ senderDisplay(detail).fallback }}</span>
      <span class="ddate">{{ formatDate(detail.date, localeStore.current) }}</span>
    </div>

    <h2 class="subject">{{ detail.subject || t['mail.noSubject'] }}</h2>

    <div class="meta">
      <RecipientList
        v-if="(detail.to || []).length"
        :label="t['search.to']"
        :addresses="detail.to"
      />
      <RecipientList
        v-if="(detail.cc || []).length"
        :label="t['search.cc']"
        :addresses="detail.cc"
      />
    </div>
  </div>
</template>

<style scoped>
.detail-header {
  padding: 18px 22px 14px;
  border-bottom: 1px solid var(--n-border-color);
}
.sender {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 2px;
  min-width: 0;
}
.sender-trigger {
  display: inline-flex;
  align-items: baseline;
  gap: 3px;
  max-width: 100%;
  margin-left: -4px;
  padding: 2px 4px;
  color: inherit;
  text-align: left;
  background: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}
.sender-trigger:hover,
.sender-trigger[data-state='open'] {
  background: var(--n-sidebar-hover);
}
.sender-chevron {
  flex: none;
  font-size: 14px;
  opacity: 0.42;
}
.from {
  font-size: 14px;
  font-weight: 600;
}
.addr {
  font-size: 13px;
  font-weight: 400;
  opacity: 0.65;
  margin-left: 4px;
}
.ddate {
  font-size: 12px;
  opacity: 0.6;
}
.subject {
  font-size: 20px;
  font-weight: 700;
  margin: 12px 0 8px;
  line-height: 1.3;
  word-break: break-word;
}
.meta {
  font-size: 12px;
}

/* Portalled menu classes: Vue still stamps the teleported nodes with this scope. */
.sender-menu-pos {
  z-index: 1000;
}
.sender-menu {
  min-width: 180px;
  padding: 4px;
  color: var(--n-text-color);
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-md);
  box-shadow: 0 8px 20px rgb(0 0 0 / 18%);
}
.sender-menu__item {
  display: grid;
  grid-template-columns: 16px minmax(0, 1fr);
  align-items: center;
  gap: 8px;
  width: 100%;
  height: 30px;
  padding: 0 8px;
  color: var(--n-text-color);
  font-size: 13px;
  background: transparent;
  border-radius: 4px;
  cursor: pointer;
  outline: none;
  user-select: none;
}
.sender-menu__item[data-highlighted] {
  background: var(--n-sidebar-hover);
}
.sender-menu__item[data-disabled] {
  color: var(--color-text-3);
  cursor: default;
}
.sender-menu__item > i {
  font-size: 16px;
  line-height: 1;
}
</style>
