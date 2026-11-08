<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { Menu } from '@ark-ui/vue/menu'
import { Clipboard } from '@wailsio/runtime'
import { useI18n } from '~/i18n'
import { useContactsStore } from '~/stores/contacts'
import { useFeedbackStore } from '~/stores/feedback'

/**
 * One address with its "copy / add to contacts" menu.
 *
 * The reading pane's sender block used to own this markup and behavior; the To/Cc
 * rows need the same menu per address, so both render through this component and
 * cannot drift apart. `variant` only changes the trigger's box: the sender sits on
 * its own line, while recipients sit in a comma-separated run and have to stay
 * inline with the surrounding text.
 */
const props = withDefaults(
  defineProps<{
    email: string
    name?: string
    variant?: 'sender' | 'inline'
  }>(),
  { name: '', variant: 'inline' },
)

const { t } = useI18n()
const contactsStore = useContactsStore()
const feedback = useFeedbackStore()

const isContact = computed(() => Boolean(props.email) && contactsStore.hasEmail(props.email))

async function copyEmail() {
  if (!props.email) return
  try {
    await Clipboard.SetText(props.email)
    feedback.success(t('contacts.copied'))
  } catch (error) {
    console.error('Failed to copy email', error)
    feedback.error(t('contacts.copyFailed'))
  }
}

async function addContact() {
  if (!props.email || isContact.value) return
  const saved = await contactsStore.addContact(props.name, props.email)
  if (!saved) {
    feedback.error(t('contacts.saveFailed'))
    return
  }
  feedback.success(t('contacts.added'))
}

async function onSelect(details: unknown) {
  const value = (details as { value?: string }).value
  if (value === 'copy-email') await copyEmail()
  if (value === 'add-contact') await addContact()
}

onMounted(() => {
  // Guarded by the store, so the per-recipient instances cost one load.
  void contactsStore.loadContacts()
})
</script>

<template>
  <Menu.Root
    v-if="props.email"
    :positioning="{ placement: 'bottom-start', gutter: 6 }"
    lazy-mount
    unmount-on-exit
    @select="onSelect"
  >
    <Menu.Trigger
      :class="['address-trigger', `address-trigger--${props.variant}`]"
      :aria-label="props.email"
    >
      <slot />
      <i class="i-ri:arrow-down-s-line address-chevron" aria-hidden="true" />
    </Menu.Trigger>
    <Teleport to="body">
      <Menu.Positioner class="address-menu__pos">
        <Menu.Content class="address-menu">
          <Menu.Item value="copy-email" class="address-menu__item">
            <i class="i-ri:file-copy-line" aria-hidden="true" />
            <Menu.ItemText>{{ t('contacts.copyEmail') }}</Menu.ItemText>
          </Menu.Item>
          <Menu.Item value="add-contact" class="address-menu__item" :disabled="isContact">
            <i :class="isContact ? 'i-ri:check-line' : 'i-ri:user-add-line'" aria-hidden="true" />
            <Menu.ItemText>
              {{ isContact ? t('contacts.alreadySaved') : t('contacts.addSender') }}
            </Menu.ItemText>
          </Menu.Item>
        </Menu.Content>
      </Menu.Positioner>
    </Teleport>
  </Menu.Root>
  <span v-else><slot /></span>
</template>

<style scoped>
.address-trigger {
  display: inline-flex;
  align-items: baseline;
  gap: 3px;
  max-width: 100%;
  margin: 0;
  padding: 0;
  color: inherit;
  font: inherit;
  line-height: inherit;
  text-align: left;
  background: transparent;
  border: none;
  border-radius: 4px;
  cursor: pointer;
  user-select: text;
}
.address-trigger:hover,
.address-trigger[data-state='open'] {
  background: var(--n-sidebar-hover);
}
/* Sender: its own block, with the same negative margin that lines the text up with
   the header's padding. */
.address-trigger--sender {
  margin-left: -4px;
  padding: 2px 4px;
}
/* Recipient: sits in the comma-separated run, so no box of its own. */
.address-trigger--inline {
  border-radius: 3px;
}
.address-chevron {
  flex: none;
  font-size: 14px;
  opacity: 0.42;
}
.address-trigger--inline .address-chevron {
  font-size: 12px;
  opacity: 0.38;
}

/* Portalled menu classes: Vue still stamps the teleported nodes with this scope. */
.address-menu__pos {
  z-index: 1000;
}
.address-menu {
  min-width: 180px;
  padding: 4px;
  color: var(--n-text-color);
  background: var(--color-surface);
  border: 1px solid var(--n-border-color);
  border-radius: var(--radius-md);
  box-shadow: 0 8px 20px rgb(0 0 0 / 18%);
}
.address-menu__item {
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
.address-menu__item[data-highlighted] {
  background: var(--n-sidebar-hover);
}
.address-menu__item[data-disabled] {
  color: var(--color-text-3);
  cursor: default;
}
.address-menu__item > i {
  font-size: 16px;
  line-height: 1;
}
</style>
