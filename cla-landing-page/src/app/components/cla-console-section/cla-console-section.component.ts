// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

import { Component, Input, OnInit } from '@angular/core';
import { AppSettings } from 'src/app/config/app-settings';
import { StorageService } from 'src/app/core/services/storage.service';
import { EnvConfig } from 'src/app/config/cla-env-utils';
import { LandingPageService } from 'src/app/service/landing-page.service';
import { environment } from 'src/environments/environment';

@Component({
  selector: 'app-cla-console-section',
  templateUrl: './cla-console-section.component.html',
  styleUrls: ['./cla-console-section.component.scss'],
})
export class ClaConsoleSectionComponent implements OnInit {
  @Input() consoleMetadata: any;
  version: string;
  links: any[];
  selectedVersion: string;
  error: string;
  consoleType: string;
  env: string;
  public environment = environment;

  constructor(
    private storageService: StorageService,
    private landingPageService: LandingPageService,
  ) {
    if (!this.landingPageService.hasEventInitilize) {
      this.landingPageService.hasEventInitilize = true;

      document.getElementById('lfx-header').addEventListener('project-login-event', () => {
        this.onClickProceed('Projects');
      });

      document.getElementById('lfx-header').addEventListener('corporate-login-event', () => {
        this.onClickProceed('Organizations');
      });
    }
  }

  ngOnInit() {
    const element: any = document.getElementById('lfx-header');
    this.links = [
      {
        title: 'Project Login',
        emit: 'project-login-event',
      },
      {
        title: 'CLA Manager Login',
        emit: 'corporate-login-event',
      },
      {
        title: 'Developer',
        url: AppSettings.CONTRIBUTORS_LEARN_MORE,
      },
    ];
    element.links = this.links;
  }

  onClickProceed(type: string) {
    this.consoleType = type;
    this.storageService.setItem('type', type);
    this.redirectAsPerTypeAndVersion(this.consoleType);
  }

  onClickLearnMore() {
    window.open(AppSettings.CONTRIBUTORS_LEARN_MORE, '_blank');
  }

  redirectAsPerTypeAndVersion(type: string) {
    const  projectConsoleUrl = EnvConfig.default[AppSettings.PROJECT_CONSOLE_LINK_V2];
    const  corporateConsoleUrl = EnvConfig.default[AppSettings.CORPORATE_CONSOLE_LINK_V2];
 
    if (type === 'Projects') {
      window.open(projectConsoleUrl, '_self');
    }
    if (type === 'Organizations') {
      window.open(corporateConsoleUrl, '_self');
    }
  }
}
