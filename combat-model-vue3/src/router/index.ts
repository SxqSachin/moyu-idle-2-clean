import { createRouter, createWebHashHistory } from 'vue-router';

const SPECIAL_VIEWS = ['scoring', 'balance', 'drop_pricing', 'enhance', 'cat_village_rep_curve', 'cat_village_rep_calc', 'difficulty', 'battle_exp_curve', 'alchemy_review', 'alchemy_inherit_review'];

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: '/',
      redirect: '/skills'
    },
    {
      path: '/:table/:id?',
      component: { template: '<div></div>' },
      props: true
    }
  ]
});

export { SPECIAL_VIEWS };
export default router;
