<template>
  <wrap
    v-bind="$props"
    v-on="$listeners"
  >
    <label
      v-if="error"
      class="text-primary p-3"
    >
      {{ error }}
    </label>

    <div
      v-else
      class="rt-content p-3"
    >
      <p
        :style="{ 'white-space': 'pre-wrap' }"
        v-html="contentBody"
      />
    </div>
  </wrap>
</template>
<script>
import base from './base'
import { evaluatePrefilter } from 'corteza-webapp-compose/src/lib/record-filter'
import { NoID } from '@cortezaproject/corteza-js'

export default {
  extends: base,

  data () {
    return {
      error: null,
      contentBody: '',
    }
  },

  watch: {
    'options.body': {
      immediate: true,
      handler () {
        this.makeContentBody()
      },
    },
  },

  methods: {
    makeContentBody () {
      this.error = null

      try {
        const { body = '' } = this.options

        this.contentBody = evaluatePrefilter(body, {
          record: this.record,
          user: this.$auth.user || {},
          recordID: (this.record || {}).recordID || NoID,
          ownerID: (this.record || {}).ownedBy || NoID,
          userID: (this.$auth.user || {}).userID || NoID,
        })
      } catch (e) {
        this.error = this.getToastMessage(e)
      }
    },
  },
}
</script>
