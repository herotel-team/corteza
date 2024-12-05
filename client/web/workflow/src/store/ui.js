const types = {
  pending: 'pending',
  completed: 'completed',
  updateLabelSet: 'updateLabelSet',
}

export default function (AutomationAPI) {
  return {
    namespaced: true,

    state: {
      pending: false,
      labels: [],
    },

    getters: {
      pending: (state) => state.pending,
      labels: (state) => state.labels,
    },

    actions: {
      async loadLabels ({ commit }) {
        commit(types.pending)
        return AutomationAPI.labelList().then(({ set }) => {
          commit(types.updateLabelSet, set)
        }).finally(() => {
          commit(types.completed)
        })
      },
    },

    mutations: {
      [types.pending] (state) {
        state.pending = true
      },

      [types.completed] (state) {
        state.pending = false
      },

      [types.updateLabelSet] (state, set) {
        state.set = set
      },
    },
  }
}
