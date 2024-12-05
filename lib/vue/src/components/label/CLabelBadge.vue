<template>
  <b-badge
    :variant="variant"
    class="label-badge"
  >
    {{ value }}
  </b-badge>
</template>

<script>
export default {
  name: 'CLabelBadge',

  props: {
    value: {
      type: String,
      required: true,
    },

    variant: {
      type: String,
      default: 'light',
    },
  },
}
</script>

<style scoped>
.label-badge {
  font-size: 100%;
  font-family: var(--font-medium);
}
</style>
