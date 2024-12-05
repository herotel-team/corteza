<template>
  <div>
    <b-form-group
      :label="$t('name.label')"
      label-class="text-primary"
    >
      <b-form-input
        v-model="workflow.meta.name"
        data-test-id="input-label"
        :placeholder="$t('name.placeholder')"
        :state="nameState"
        @input="$root.$emit('change-detected')"
      />
    </b-form-group>

    <b-form-group
      :label="$t('handle.label')"
      label-class="text-primary"
    >
      <b-form-input
        v-model="workflow.handle"
        data-test-id="input-handle"
        :state="handleState"
        :placeholder="$t('handle.placeholder')"
        @input="$root.$emit('change-detected')"
      />
      <b-form-invalid-feedback
        data-test-id="input-handle-invalid-state"
        :state="handleState"
      >
        {{ $t('handle.invalid-handle-characters') }}
      </b-form-invalid-feedback>
    </b-form-group>

    <b-form-group
      :label="$t('description.label')"
      label-class="text-primary"
    >
      <b-form-textarea
        v-model="workflow.meta.description"
        data-test-id="input-description"
        :placeholder="$t('description.placeholder')"
        @input="$root.$emit('change-detected')"
      />
    </b-form-group>

    <b-form-group
      :label="$t('labels.label')"
      label-class="text-primary"
    >
      <b-input-group>
        <c-input-select
          v-model="workflowLabels"
          :options="availableLabels"
          :placeholder="$t('labels.placeholder')"
          :get-option-label="l => l"
          multiple
        />

        <b-input-group-append>
          <b-button
            v-b-tooltip.hover="{ title: $t('labels.addNewLabel.tooltip'), container: '#body' }"
            variant="light"
            class="d-flex align-items-center"
            @click="$bvModal.show('addNewLabel')"
          >
            <font-awesome-icon
              :icon="['fas', 'plus']"
              class="text-primary"
            />
          </b-button>
        </b-input-group-append>
      </b-input-group>
    </b-form-group>

    <b-form-group
      :label="$t('run-as.label')"
      :description="$t('run-as.description')"
      label-class="text-primary"
    >
      <c-input-select
        data-test-id="select-run-as"
        :options="user.options"
        :get-option-label="getUserLabel"
        :get-option-key="getUserKey"
        :value="user.value"
        :placeholder="$t('run-as.placeholder')"
        :filterable="false"
        @search="search"
        @input="updateRunAs"
      />
    </b-form-group>

    <b-form-group>
      <b-form-checkbox
        v-model="workflow.enabled"
        data-test-id="checkbox-enable-workflow"
        @change="$root.$emit('change-detected')"
      >
        {{ $t('general:enabled') }}
      </b-form-checkbox>
    </b-form-group>

    <b-form-group
      :description="$t('sub-workflow.description')"
    >
      <b-form-checkbox
        v-model="workflow.meta.subWorkflow"
        data-test-id="checkbox-sub-workflow"
        @change="$root.$emit('change-detected')"
      >
        {{ $t('sub-workflow.label') }}
      </b-form-checkbox>
    </b-form-group>

    <b-modal
      id="addNewLabel"
      :title="$t('labels.addNewLabel.modal.title')"
      centered
      @cancel="newLabel = ''"
      @ok="addNewLabel"
    >
      <b-form-input
        v-model="newLabel"
        :placeholder="$t('labels.addNewLabel.modal.placeholder')"
      />

      <template #modal-footer="{ ok, cancel }">
        <b-button
          variant="light"
          @click="cancel"
        >
          {{ $t('general:cancel') }}
        </b-button>

        <b-button
          variant="primary"
          @click="ok"
        >
          {{ $t('general:save') }}
        </b-button>
      </template>
    </b-modal>
  </div>
</template>

<script>
import { debounce } from 'lodash'
import { handle } from '@cortezaproject/corteza-vue'

export default {
  i18nOptions: {
    namespaces: 'configurator',
  },

  props: {
    workflow: {
      type: Object,
      default: () => {},
    },
  },

  data () {
    return {
      user: {
        options: [],
        value: undefined,

        filter: {
          query: null,
          limit: 10,
        },
      },

      allLabels: [],

      newLabel: '',
    }
  },

  computed: {
    nameState () {
      return this.workflow.meta.name ? null : false
    },

    handleState () {
      return handle.handleState(this.workflow.handle)
    },

    availableLabels () {
      return this.allLabels.map(l => l[Object.keys(l)[0]])
    },

    workflowLabels: {
      get () {
        return Object.keys(this.workflow.labels || {})
      },

      set (labels) {
        this.$set(this.workflow, 'labels', labels.reduce((acc, label) => {
          acc[label] = label
          return acc
        }, {}))
      },
    },
  },

  created () {
    if (this.workflow.runAs) {
      this.fetchUsers()
      this.getUserByID()
    }
  },

  methods: {
    search: debounce(function (query) {
      if (query !== this.user.filter.query) {
        this.user.filter.query = query
        this.user.filter.page = 1
      }

      if (query) {
        this.fetchUsers()
      }
    }, 300),

    fetchUsers () {
      this.$SystemAPI.userList(this.user.filter)
        .then(({ set }) => {
          this.user.options = set.map(m => Object.freeze(m))
        })
    },

    async getUserByID () {
      if (this.workflow.runAs !== '0') {
        this.$SystemAPI.userRead({ userID: this.workflow.runAs })
          .then(user => {
            this.user.value = user
          }).catch(() => {
            return {}
          })
      }
    },

    updateRunAs (user) {
      if (user && user.userID) {
        this.user.value = user
        this.workflow.runAs = user.userID
      } else {
        this.user.value = null
        this.workflow.runAs = '0'
      }
      this.$root.$emit('change-detected')
    },

    getUserKey ({ userID }) {
      return userID
    },

    getUserLabel ({ userID, email, name, username }) {
      return name || username || email || `<@${userID}>`
    },

    addNewLabel () {
      if (this.newLabel && !this.allLabels.includes(this.newLabel)) {
        this.allLabels.push({ [this.newLabel]: this.newLabel })
        this.workflowLabels = [...this.workflowLabels, this.newLabel]
      }

      this.newLabel = ''
      this.$bvModal.hide('addNewLabel')
    },
  },
}
</script>
