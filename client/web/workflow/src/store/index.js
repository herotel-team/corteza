import Vue from 'vue'
import Vuex from 'vuex'

import ui from './ui'

import { store as cvStore } from '@cortezaproject/corteza-vue'

Vue.use(Vuex)

const store = new Vuex.Store({
  modules: {
    ui: ui(Vue.prototype.$AutomationAPI),
    rbac: {
      namespaced: true,
      ...cvStore.RBAC(Vue.prototype.$AutomationAPI),
    },
  },
})

export default store
