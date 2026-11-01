import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { TagService } from '#/wvmail/services'
import type { EmailSummary } from '~/types/email'
import type { MailTag } from '~/types/tag'

export const useTagsStore = defineStore('tags', () => {
  const tags = ref<MailTag[]>([])
  const tagsLoading = ref(false)
  const selectedFilterIDs = ref<string[]>([])
  const emails = ref<EmailSummary[]>([])
  const total = ref(0)
  const page = ref(1)
  const pageSize = 20
  const listLoading = ref(false)
  let filterInitialized = false
  let listRequestId = 0

  const enabledTags = computed(() => tags.value.filter((tag) => tag.enabled))
  const tagByID = computed(() => {
    const map: Record<string, MailTag> = {}
    for (const tag of tags.value) map[tag.id] = tag
    return map
  })
  const selectedTags = computed(() =>
    selectedFilterIDs.value.map((id) => tagByID.value[id]).filter(Boolean),
  )

  async function loadTags() {
    tagsLoading.value = true
    try {
      tags.value = (await TagService.ListMailTags()) as MailTag[]
      if (!filterInitialized) {
        selectedFilterIDs.value = enabledTags.value.map((tag) => tag.id)
        filterInitialized = true
      } else {
        const existing = new Set(tags.value.map((tag) => tag.id))
        selectedFilterIDs.value = selectedFilterIDs.value.filter((id) => existing.has(id))
      }
      return tags.value
    } finally {
      tagsLoading.value = false
    }
  }

  async function loadEmails(nextPage = page.value) {
    const requestId = ++listRequestId
    page.value = Math.max(1, nextPage)
    if (selectedFilterIDs.value.length === 0) {
      emails.value = []
      total.value = 0
      listLoading.value = false
      return
    }
    listLoading.value = true
    try {
      const response = await TagService.ListClassifiedEmails(
        selectedFilterIDs.value,
        page.value,
        pageSize,
      )
      if (requestId !== listRequestId) return
      emails.value = (response?.emails ?? []) as EmailSummary[]
      total.value = response?.total ?? 0
      const pages = Math.max(1, Math.ceil(total.value / pageSize))
      if (page.value > pages) {
        await loadEmails(pages)
      }
    } finally {
      if (requestId === listRequestId) listLoading.value = false
    }
  }

  async function setFilterIDs(ids: string[]) {
    selectedFilterIDs.value = [...new Set(ids)]
    await loadEmails(1)
  }

  async function reload() {
    await loadTags()
    await loadEmails(page.value)
  }

  function setEmailRead(accountID: string, folder: string, uid: number) {
    const row = emails.value.find(
      (email) =>
        email.account_id === accountID && email.folder_name === folder && email.uid === uid,
    )
    if (row) row.is_read = true
  }

  function removeEmail(accountID: string, folder: string, uid: number) {
    const next = emails.value.filter(
      (email) =>
        !(email.account_id === accountID && email.folder_name === folder && email.uid === uid),
    )
    if (next.length !== emails.value.length) {
      emails.value = next
      total.value = Math.max(0, total.value - 1)
    }
  }

  function setEmailTagIDs(
    accountID: string,
    folder: string,
    uid: number,
    selectedTagIDs: string[],
  ) {
    const row = emails.value.find(
      (email) =>
        email.account_id === accountID && email.folder_name === folder && email.uid === uid,
    )
    if (!row) return
    row.tag_ids = selectedTagIDs
    if (!selectedTagIDs.some((id) => selectedFilterIDs.value.includes(id))) {
      removeEmail(accountID, folder, uid)
    }
  }

  return {
    tags,
    tagsLoading,
    enabledTags,
    tagByID,
    selectedFilterIDs,
    selectedTags,
    emails,
    total,
    page,
    pageSize,
    listLoading,
    loadTags,
    loadEmails,
    setFilterIDs,
    reload,
    setEmailRead,
    removeEmail,
    setEmailTagIDs,
  }
})
