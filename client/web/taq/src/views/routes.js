export default [
  {
    path: '/',
    name: 'root',
    component: () => import('./Layout'),
    redirect: 'builder',
    children: [
      {
        path: 'builder',
        name: 'builder',
        component: () => import('./Builder/index.vue'),
      },
    ],
  },

  { path: '*', redirect: { name: 'root' } },
]
