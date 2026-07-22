import type { Routes } from '@angular/router';

import { authGuard, anonymousOnlyGuard } from './core/auth/auth.guard';
import { i18nNamespaces } from './core/i18n/i18n.route';

export const routes: Routes = [
  {
    path: 'login',
    canActivate: [anonymousOnlyGuard],
    resolve: { translations: i18nNamespaces(['auth']) },
    loadComponent: () => import('./features/auth/login.page').then((module) => module.LoginPage),
  },
  {
    path: 'register',
    canActivate: [anonymousOnlyGuard],
    resolve: { translations: i18nNamespaces(['identity']) },
    loadComponent: () =>
      import('./features/auth/registration.page').then((module) => module.RegistrationPage),
  },
  {
    path: 'password-reset',
    canActivate: [anonymousOnlyGuard],
    resolve: { translations: i18nNamespaces(['identity']) },
    loadComponent: () =>
      import('./features/auth/password-reset.page').then((module) => module.PasswordResetPage),
  },
  {
    path: '',
    canActivate: [authGuard],
    loadComponent: () =>
      import('./shell/app-shell.component').then((module) => module.AppShellComponent),
    children: [
      { path: '', pathMatch: 'full', redirectTo: 'dashboard' },
      {
        path: 'dashboard',
        resolve: { translations: i18nNamespaces(['dashboard', 'activities']) },
        loadComponent: () =>
          import('./features/dashboard/dashboard.page').then((module) => module.DashboardPage),
      },
      {
        path: 'contacts',
        resolve: { translations: i18nNamespaces(['contacts']) },
        loadComponent: () =>
          import('./features/contacts/contacts.page').then((module) => module.ContactsPage),
      },
      {
        path: 'contacts/:id',
        resolve: { translations: i18nNamespaces(['contacts', 'activities', 'files']) },
        loadComponent: () =>
          import('./features/contacts/contact-details.page').then(
            (module) => module.ContactDetailsPage,
          ),
      },
      {
        path: 'companies',
        resolve: { translations: i18nNamespaces(['companies']) },
        loadComponent: () =>
          import('./features/companies/companies.page').then((module) => module.CompaniesPage),
      },
      {
        path: 'companies/:id',
        resolve: { translations: i18nNamespaces(['companies', 'activities', 'files']) },
        loadComponent: () =>
          import('./features/companies/company-details.page').then(
            (module) => module.CompanyDetailsPage,
          ),
      },
      {
        path: 'leads',
        resolve: { translations: i18nNamespaces(['leads', 'assignments']) },
        loadComponent: () =>
          import('./features/leads/leads.page').then((module) => module.LeadsPage),
      },
      {
        path: 'deals',
        resolve: { translations: i18nNamespaces(['sales']) },
        loadComponent: () =>
          import('./features/deals/deals.page').then((module) => module.DealsPage),
      },
      {
        path: 'deals/:id',
        resolve: { translations: i18nNamespaces(['sales', 'activities', 'files', 'assignments']) },
        loadComponent: () =>
          import('./features/deals/deal-details.page').then((module) => module.DealDetailsPage),
      },
      {
        path: 'projects',
        resolve: { translations: i18nNamespaces(['projects']) },
        loadComponent: () =>
          import('./features/projects/projects.page').then((module) => module.ProjectsPage),
      },
      {
        path: 'projects/:id',
        resolve: { translations: i18nNamespaces(['projects', 'activities', 'files']) },
        loadComponent: () =>
          import('./features/projects/project-details.page').then(
            (module) => module.ProjectDetailsPage,
          ),
      },
      {
        path: 'activities',
        resolve: { translations: i18nNamespaces(['activities', 'assignments']) },
        loadComponent: () =>
          import('./features/activities/activities.page').then((module) => module.ActivitiesPage),
      },
      {
        path: 'calendar',
        resolve: { translations: i18nNamespaces(['calendar', 'activities']) },
        loadComponent: () =>
          import('./features/calendar/calendar.page').then((module) => module.CalendarPage),
      },
      {
        path: 'automations',
        resolve: { translations: i18nNamespaces(['automations']) },
        loadComponent: () =>
          import('./features/automations/automations.page').then(
            (module) => module.AutomationsPage,
          ),
      },
      {
        path: 'reports',
        resolve: { translations: i18nNamespaces(['reports']) },
        loadComponent: () =>
          import('./features/reports/reports.page').then((module) => module.ReportsPage),
      },
      {
        path: 'notifications',
        resolve: { translations: i18nNamespaces(['notifications']) },
        loadComponent: () =>
          import('./features/notifications/notifications.page').then(
            (module) => module.NotificationsPage,
          ),
      },
      {
        path: 'workspace/new',
        resolve: { translations: i18nNamespaces(['identity']) },
        loadComponent: () =>
          import('./features/workspaces/workspace-create.page').then(
            (module) => module.WorkspaceCreatePage,
          ),
      },
      {
        path: 'invitations/accept',
        resolve: { translations: i18nNamespaces(['identity']) },
        loadComponent: () =>
          import('./features/workspaces/invitation-accept.page').then(
            (module) => module.InvitationAcceptPage,
          ),
      },
      {
        path: 'settings',
        resolve: { translations: i18nNamespaces(['settings']) },
        loadComponent: () =>
          import('./features/settings/settings.page').then((module) => module.SettingsPage),
      },
      {
        path: 'settings/security',
        resolve: { translations: i18nNamespaces(['identity']) },
        loadComponent: () =>
          import('./features/security/security.page').then((module) => module.SecurityPage),
      },
      {
        path: 'settings/members',
        resolve: { translations: i18nNamespaces(['members']) },
        loadComponent: () =>
          import('./features/members/members.page').then((module) => module.MembersPage),
      },
      {
        path: 'settings/roles',
        resolve: { translations: i18nNamespaces(['members', 'roles']) },
        loadComponent: () =>
          import('./features/roles/roles.page').then((module) => module.RolesPage),
      },
      {
        path: 'settings/lead-stages',
        resolve: { translations: i18nNamespaces(['leadStages']) },
        loadComponent: () =>
          import('./features/lead-stages/lead-stages.page').then((module) => module.LeadStagesPage),
      },
      {
        path: 'settings/pipelines',
        resolve: { translations: i18nNamespaces(['pipelines']) },
        loadComponent: () =>
          import('./features/pipeline-settings/pipeline-settings.page').then(
            (module) => module.PipelineSettingsPage,
          ),
      },
      {
        path: 'settings/custom-fields',
        resolve: { translations: i18nNamespaces(['customFields']) },
        loadComponent: () =>
          import('./features/custom-fields/custom-fields.page').then(
            (module) => module.CustomFieldsPage,
          ),
      },
      {
        path: 'settings/api',
        resolve: { translations: i18nNamespaces(['integrations']) },
        loadComponent: () =>
          import('./features/api-keys/api-keys.page').then((module) => module.ApiKeysPage),
      },
      {
        path: 'settings/webhooks',
        resolve: { translations: i18nNamespaces(['integrations']) },
        loadComponent: () =>
          import('./features/webhooks/webhooks.page').then((module) => module.WebhooksPage),
      },
      {
        path: 'settings/audit',
        loadComponent: () =>
          import('./features/audit/audit.page').then((module) => module.AuditPage),
      },
      {
        path: 'settings/localization',
        resolve: { translations: i18nNamespaces(['translations']) },
        loadComponent: () =>
          import('./features/translations/workspace-localization.page').then(
            (module) => module.WorkspaceLocalizationPage,
          ),
      },
      {
        path: 'settings/translations',
        resolve: { translations: i18nNamespaces(['translations']) },
        loadComponent: () =>
          import('./features/translations/translations.page').then(
            (module) => module.TranslationsPage,
          ),
      },
      { path: '**', redirectTo: 'dashboard' },
    ],
  },
];
