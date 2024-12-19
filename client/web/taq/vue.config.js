const buildVueConfig = require('./vue.config-builder')

module.exports = buildVueConfig({
  appFlavour: 'TAQ',
  appName: 'taq',
  appLabel: 'Corteza TAQ',
  theme: 'corteza-base',
  packageAlias: 'corteza-webapp-taq',
})
