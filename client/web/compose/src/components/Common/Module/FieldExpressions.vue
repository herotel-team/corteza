<template>
  <b-form-group
    class="p-0 m-0"
  >
    <b-form-row
      v-for="(expr, ei) in value"
      :key="ei"
      class="mb-2"
      no-gutters
    >
      <b-input-group>
        <b-input-group-prepend>
          <b-button
            v-b-tooltip.noninteractive.hover="{ title: $t('validators.expression.tooltip'), container: '#body' }"
            variant="extra-light"
          >
            ƒ
          </b-button>
        </b-input-group-prepend>
        <slot :value="value[ei]">
          <b-form-input
            v-model="value[ei]"
            :placeholder="placeholder"
          />
        </slot>
        <b-input-group-addon
          class="m-1"
        >
          <!-- no prompt/confirmation on empty input -->
          <c-input-confirm
            :no-prompt="noPrompt(value[ei])"
            show-icon
            @confirmed="$emit('remove', ei)"
          />
        </b-input-group-addon>
      </b-input-group>
    </b-form-row>
  </b-form-group>
</template>
<script>

export default {
  i18nOptions: {
    namespaces: 'field',
  },

  props: {
    value: {
      type: Array,
      default: () => ([]),
    },

    placeholder: {
      type: String,
      default: () => {},
    },

    noPrompt: {
      type: Function,
      default: v => v.length === 0,
    },
  },
}
</script>
