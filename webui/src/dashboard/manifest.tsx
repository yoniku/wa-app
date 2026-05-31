import { Smartphone } from 'lucide-react';
import { DashboardNavSection, type DashboardModuleRegistration } from '@byte-v-forge/common-ui';
import { WaPage } from './wa-page';

const registration: DashboardModuleRegistration = {
  manifest: {
    id: 'wa-app',
    nav: [
      {
        key: 'wa',
        label: 'WA 管理',
        icon: 'wa-app',
        section: DashboardNavSection.DASHBOARD_NAV_SECTION_MAIN,
        required_services: ['wa-app-service'],
        order: 18
      }
    ]
  },
  icons: { 'wa-app': <Smartphone size={17} /> },
  views: { wa: () => <WaPage /> }
};

export default registration;
