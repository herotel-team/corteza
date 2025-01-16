<template>
  <div>
    <span
      v-for="(v, index) of value"
      :key="index"
      :class="{ 'd-block mb-2': field.options.multiDelimiter === '\n' }"
    >
      <span
        :class="{ 'badge badge-pill': field.options.displayType === 'badge' }"
        :style="v.style"
      >
        {{ v.text }}
      </span>

      {{ index !== value.length - 1 ? field.options.multiDelimiter : '' }}
    </span>

    <errors :errors="errors" />
  </div>
</template>

<script>
import base from './base'

export default {
  extends: base,

  computed: {
    /**
     * Overwrite default; allow values to resolve to their labels
     * @returns {String|Array<String>}
     */
    value () {
      let v
      if (this.field.isSystem) {
        v = this.record[this.field.name]
      }
      v = this.record ? this.record.values[this.field.name] : undefined

      if (this.field.isMulti) {
        if (!Array.isArray(v)) {
          v = []
        }

        return v.map(v => this.resolveValue(v) || v).filter(Boolean)
      } else {
        return [this.resolveValue(v) || v].filter(Boolean)
      }
    },
  },

  methods: {
    resolveValue (v) {
      const opt = this.field.options.options.find(({ value }) => value === v) || { text: v }

      return {
        text: opt.text,
        style: this.getOptionStyle(opt),
      }
    },

    getOptionStyle (opt) {
      const style = {}

      if (this.field.options.displayType === 'badge') {
        style.fontSize = '0.9rem'
        style.color = opt.style.textColor || 'var(--dark)'
        style.backgroundColor = opt.style.backgroundColor || 'var(--extra-light)'
      }

      return style
    },
  },
}
</script>
