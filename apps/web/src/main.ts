import { bootstrapApplication } from '@angular/platform-browser';
import { productConfig } from '@veltrix-crm/product-config';
import { appConfig } from './app/app.config';
import { App } from './app/app';

document.title = productConfig.productName;
bootstrapApplication(App, appConfig).catch((err) => console.error(err));
