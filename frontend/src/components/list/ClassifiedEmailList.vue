<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from '~/i18n'
import { useAccountsStore } from '~/stores/accounts'
import { useTagsStore } from '~/stores/tags'
import EmailListHeader from './EmailListHeader.vue'
import EmailListItem from './EmailListItem.vue'
import TagFilter from '~/components/tags/TagFilter.vue'
import UiScrollArea from '~/components/ui/UiScrollArea.vue'

const { t } = useI18n()
const accountsStore = useAccountsStore()
const tagsStore = useTagsStore()

const loading = computed(() => tagsStore.tagsLoading || tagsStore.listLoading)
const pageCount = computed(() =>
  tagsStore.total > 0 ? Math.ceil(tagsStore.total / tagsStore.pageSize) : 1,
)
const emptyMessage = computed(() => {
  if (tagsStore.tags.length === 0) return t.value['tags.noTagsYet']
  if (tagsStore.selectedFilterIDs.length === 0) return t.value['tags.selectTags']
  return t.value['tags.noMatch']
})
const selectedEmailKey = computed(() => {
  const uid = accountsStore.selectedEmailUid
  const accountId = accountsStore.selectedEmailAccountId
  const folder = accountsStore.selectedEmailFolder
  return uid != null && accountId && folder ? `${accountId}\u200b${folder}\u200b${uid}` : ''
})

function emailKey(email: { uid: number; account_id?: string; folder_name?: string }): string {
  return email.account_id && email.folder_name
    ? `${email.account_id}\u200b${email.folder_name}\u200b${email.uid}`
    : ''
}

function isSelected(email: { uid: number; account_id?: string; folder_name?: string }): boolean {
  return selectedEmailKey.value !== '' && selectedEmailKey.value === emailKey(email)
}

function selectEmail(email: { uid: number; account_id?: string; folder_name?: string }) {
  if (!email.account_id || !email.folder_name) return
  const account = accountsStore.accounts.find((item) => item.ID === email.account_id)
  if (account) accountsStore.selectEmail(account, email.folder_name, email.uid)
}

onMounted(async () => {
  await tagsStore.loadTags()
  await tagsStore.loadEmails(1)
})
</script>

<template>
  <div class="email-list">
    <EmailListHeader
      :label="t['tags.classified']"
      :title="t['tags.classified']"
      :show-refresh="false"
      :refreshing="false"
      :page-count="pageCount"
      :total="tagsStore.total"
      :page-size="tagsStore.pageSize"
      :page="tagsStore.page"
      @refresh="tagsStore.loadEmails(tagsStore.page)"
      @page-change="tagsStore.loadEmails"
    >
      <template #filters>
        <TagFilter
          :selected="tagsStore.selectedFilterIDs"
          @change="(ids) => void tagsStore.setFilterIDs(ids)"
        />
      </template>
    </EmailListHeader>

    <UiScrollArea class="body">
      <div v-if="loading" class="spin-row"><span class="spinner" /></div>
      <p v-else-if="tagsStore.emails.length === 0" class="empty">{{ emptyMessage }}</p>
      <template v-else>
        <EmailListItem
          v-for="email in tagsStore.emails"
          :key="`${email.account_id}:${email.folder_name}:${email.uid}`"
          :email="email"
          :selected="isSelected(email)"
          smart
          @select="selectEmail"
        />
      </template>
    </UiScrollArea>
  </div>
</template>

<style scoped>
.email-list {
  height: 100%;
  width: 100%;
  display: flex;
  flex-direction: column;
  min-width: 0;
  background: var(--n-body-color);
}
.body {
  flex: 1;
  min-height: 0;
}
.spin-row {
  padding: 32px 0;
  display: flex;
  justify-content: center;
}
.empty {
  padding: 40px 20px;
  text-align: center;
  color: var(--color-text-3);
  font-size: 13px;
}
</style>
