<script setup lang="ts">
import { watchEffect } from 'vue'
import { useThemeVars } from 'naive-ui'

const themeVars = useThemeVars()

watchEffect(() => {
  const root = document.documentElement
  root.style.setProperty('--n-body-color', themeVars.value.bodyColor)
  root.style.setProperty('--n-text-color', themeVars.value.textColor2)
  root.style.setProperty('--n-border-color', themeVars.value.borderColor)
  root.style.setProperty('--n-sidebar-hover', 'rgba(128, 128, 128, 0.08)')
  root.style.setProperty('--n-sidebar-active', 'rgba(128, 128, 128, 0.12)')
})
</script>

<template />
