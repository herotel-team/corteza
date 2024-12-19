<template>
  <div class="taq-builder">
    <portal to="topbar-title">
      {{ $t('title') }}
    </portal>

    <split class="overflow-hidden">
      <split-area
        :size="selectedStep ? 80 : 100"
        :min-size="600"
        class="diagram-pane overflow-auto p-4 rounded"
      >
        <div style="max-width: 500px; margin: 0 auto;">
          <div class="d-flex flex-column align-items-center gap-5">
            <transition-group
              name="list"
              class="w-100"
            >
              <div
                v-for="(step, index) in steps"
                :key="step.stepID"
                class="d-flex flex-column align-items-center w-100"
              >
                <b-card
                  body-class="d-flex align-items-center pr-2"
                  class="step shadow-sm w-100 pointer border"
                  :class="{ 'border-primary': selectedStepID === step.stepID, 'shadow': hoverStepIndex === index }"
                  style="border-width: 2px !important;"
                  @click.stop="selectStep(step.stepID)"
                  @mouseenter="hoverStepIndex = index"
                  @mouseleave="hoverStepIndex = null"
                >
                  <div
                    class="d-flex align-items-center justify-content-center bg-light rounded-lg"
                    style="width: 64px; height: 64px;"
                  >
                    <font-awesome-icon
                      :icon="['far', index === 0 ? 'clock' : 'file-alt']"
                      size="3x"
                      class="text-primary"
                    />
                  </div>

                  <div class="d-flex flex-column px-4">
                    <h5>
                      {{ getStepDisplayName(index, step) }}
                    </h5>

                    <p class="mb-0 mt-1 text-muted">
                      {{ step.description }}
                    </p>
                  </div>

                  <b-dropdown
                    variant="outline-extra-light"
                    toggle-class="d-flex align-items-center justify-content-center text-primary border-0 py-2"
                    no-caret
                    lazy
                    right
                    menu-class="m-0"
                    class="ml-auto align-self-start"
                    @click.stop
                  >
                    <template #button-content>
                      <font-awesome-icon
                        :icon="['fas', 'ellipsis-v']"
                      />
                    </template>

                    <b-dropdown-item-button @click.stop="deleteStep(step.stepID)">
                      Delete
                    </b-dropdown-item-button>
                  </b-dropdown>
                </b-card>

                <div
                  v-if="step.kind === 'branch'"
                  class="d-flex flex-column align-items-center w-100"
                >
                  <div class="connector small" />

                  <div class="branch-square">
                    <div class="branch-connector left">
                      <p class="bg-light rounded p-2 mb-0">
                        True
                      </p>

                      <b-button
                        variant="extra-light"
                        size="lg"
                        class="text-light"
                      >
                        <font-awesome-icon
                          :icon="['fas', 'plus']"
                        />
                      </b-button>
                    </div>

                    <div class="branch-connector right">
                      <p class="bg-light rounded p-2 mb-0">
                        False
                      </p>

                      <b-button
                        variant="extra-light"
                        size="lg"
                        class="text-light"
                      >
                        <font-awesome-icon
                          :icon="['fas', 'plus']"
                        />
                      </b-button>
                    </div>
                  </div>
                </div>

                <div class="d-flex align-items-center justify-content-around w-100">
                  <div
                    class="connector"
                  >
                    <b-popover
                      :target="`add-step-button-${index}`"
                      triggers="click blur"
                      delay="0"
                      boundary="window"
                      boundary-padding="2"
                    >
                      <div class="d-flex flex-column p-1">
                        <div
                          class="dropdown-item d-flex align-items-center justify-content-start gap-2 pointer px-2 rounded"
                          @click="addStep(index + 1)"
                        >
                          <div
                            class="d-flex align-items-center justify-content-center bg-light rounded"
                            style="width: 32px; height: 32px;"
                          >
                            <font-awesome-icon
                              :icon="['far', 'file-alt']"
                              size="2x"
                              class="text-primary"
                            />
                          </div>
                          Create Record
                        </div>
                      </div>
                    </b-popover>

                    <b-button
                      :id="`add-step-button-${index}`"
                      variant="extra-light"
                      size="lg"
                      class="connector-button text-primary"
                    >
                      <font-awesome-icon
                        :icon="['fas', 'plus']"
                      />
                    </b-button>
                  </div>
                </div>
              </div>
            </transition-group>

            <p class="rounded bg-extra-light p-3 px-4">
              End
            </p>
          </div>
        </div>
      </split-area>

      <split-area
        :size="selectedStep ? 20 : 0"
        :min-size="350"
        class="px-1 pb-2"
      >
        <transition
          name="fade"
        >
          <div
            v-if="selectedStep"
            class="d-flex flex-column h-100"
          >
            <b-card
              header-class="d-flex align-items-center justify-content-between w-100 pr-2 border-bottom"
              class="h-100"
            >
              <template #header>
                <h5 class="mb-0">
                  {{ getSidebarTitle(selectedStepID) }}
                </h5>

                <b-button
                  variant="outline-light"
                  class="ml-auto text-primary border-0"
                  @click="closeConfigurator"
                >
                  <font-awesome-icon
                    :icon="['fas', 'times']"
                  />
                </b-button>
              </template>

              <p>{{ selectedStep.description }}</p>
            </b-card>
          </div>
        </transition>
      </split-area>
    </split>
  </div>
</template>

<script>
import { Split, SplitArea } from 'vue-split-panel'

export default {
  name: 'Builder',

  i18nOptions: {
    namespaces: 'builder',
  },

  components: {
    Split,
    SplitArea,
  },

  data () {
    return {
      steps: [],

      selectedStepID: null,
      hoverStepIndex: null,

    }
  },

  computed: {
    selectedStep () {
      return this.steps.find(step => step.stepID === this.selectedStepID)
    },
  },

  created () {
    this.steps = [
      {
        stepID: Date.now(),
        name: 'Trigger',
        description: 'Every Day',
        kind: 'trigger',
      },
    ]
  },

  methods: {
    addStep (index) {
      const newStep = {
        stepID: Date.now(),
        name: 'Branch',
        description: 'Create a new branch',
        kind: 'branch',
      }

      if (index >= 0) {
        this.steps.splice(index, 0, newStep)
      } else {
        this.steps.push(newStep)
      }
    },

    selectStep (stepID) {
      this.selectedStepID = null

      this.selectedStepID = stepID
    },

    closeConfigurator () {
      this.selectedStepID = null
    },

    deleteStep (stepID) {
      this.steps = this.steps.filter(step => step.stepID !== stepID)

      if (this.selectedStepID === stepID) {
        this.closeConfigurator()
      }
    },

    handleSidebarHidden () {
      this.selectedStepID = null
    },

    getStepDisplayName (index, step) {
      return `${index + 1}. ${step.name}`
    },

    getSidebarTitle (stepID) {
      return this.getStepDisplayName(this.steps.findIndex(step => step.stepID === stepID), this.steps.find(step => step.stepID === stepID))
    },
  },
}
</script>

<style scoped>
.step {
  transition: box-shadow 0.2s ease-in-out;
}

.connector {
  width: 3px;
  height: 100px;
  background-color: var(--extra-light);
  position: relative;

  .connector-button {
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
  }

  &.small {
    height: 50px;
  }
}

.branch-square {
  display: flex;
  align-items: stretch;
  width: 500px;
  min-height: 300px;
  border: 3px solid var(--extra-light);
  border-radius: 1rem;

  .branch-connector {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: space-around;
    width: 500px;
    padding: 2rem;
    gap: 2rem;

    &.right {
      margin-right: -250px;
    }

    &.left {
      margin-left: -250px;
    }

    .step {
      width: 100%;
    }
  }
}

.taq-sidebar-spacer {
  padding-right: 500px !important;
}

.taq-builder {
  transition: padding-right 0.3s ease-in-out;

  .diagram-pane {
    /* background-color: var(--light); */
    background-image: radial-gradient(var(--extra-light) 2px, transparent 2px);
    background-size: 25px 25px;
  }

  .gutter {
    background-color: var(--body-bg);
  }
}

.fade-enter-active, .fade-leave-active {
  transition: opacity 0.5s ease;
}

.fade-enter, .fade-leave-to /* .fade-leave-active below version 2.1.8 */ {
  opacity: 0;
}

.list-enter-active {
  transition: all 1s;
}

.list-leave-active {
  transition: all 0.2s;
}

.list-enter, .list-leave-to /* .list-leave-active below version 2.1.8 */ {
  opacity: 0;
}
</style>
