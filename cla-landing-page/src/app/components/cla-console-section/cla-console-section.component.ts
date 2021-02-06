// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

import { Component, Input, OnInit, TemplateRef, ViewChild } from '@angular/core';
import { AppSettings } from 'src/app/config/app-settings';
import { StorageService } from 'src/app/core/services/storage.service';
import { EnvConfig } from 'src/app/config/cla-env-utils';
import { ActivatedRoute } from '@angular/router';
import { NgbModal, NgbModalRef } from '@ng-bootstrap/ng-bootstrap';
import { LandingPageService } from 'src/app/service/landing-page.service';
import { environment } from 'src/environments/environment';

declare let process: any;
@Component({
  selector: 'app-cla-console-section',
  templateUrl: './cla-console-section.component.html',
  styleUrls: ['./cla-console-section.component.scss']
})
export class ClaConsoleSectionComponent implements OnInit {
  @Input() consoleMetadata: any;
  @ViewChild('versionModal') versionModal: TemplateRef<any>;
  version: string;
  links: any[];
  modelRef: NgbModalRef;
  selectedVersion: string;
  error: string;
  consoleType: string;
  env: string;
  public environment = environment;

  constructor(
    private storageService: StorageService,
    private router: ActivatedRoute,
    private modalService: NgbModal,
    private landingPageService: LandingPageService
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
    this.version = this.router.snapshot.queryParamMap.get('version');
    const element: any = document.getElementById('lfx-header');
    let projectConsoleUrl = EnvConfig.default[AppSettings.PROJECT_CONSOLE_LINK] + '#/login';
    let corporateConsoleUrl = EnvConfig.default[AppSettings.CORPORATE_CONSOLE_LINK] + '#/login'
    if (this.version === '2') {
      // Set redirect URL to new V2 console.
      projectConsoleUrl = EnvConfig.default[AppSettings.PROJECT_CONSOLE_LINK_V2];
      corporateConsoleUrl = EnvConfig.default[AppSettings.CORPORATE_CONSOLE_LINK_V2];
    }
    this.links = [
      {
        title: 'Project Login',
        emit: "project-login-event"
      },
      {
        title: 'CLA Manager Login',
        emit: "corporate-login-event"
      },
      {
        title: 'Developer',
        url: AppSettings.CONTRIBUTORS_LEARN_MORE
      }
    ];
    element.links = this.links;
  }

  onClickProceed(type: string) {
    this.consoleType = type;
    this.storageService.setItem('type', type);
    if (this.version === '1' || this.version === '2') {
      this.redirectAsPerTypeAndVersion(this.consoleType, this.version);
    } else {
      // Show version dialog
      this.openDialog();
      return false;
    }
  }

  openDialog() {
    this.error = '';
    this.selectedVersion = '';
    this.modelRef = this.modalService.open(this.versionModal, {
      centered: true,
      backdrop: 'static',
      keyboard: false
    });
  }

  onClickRequestAccess() {
    window.open(AppSettings.REQUEST_ACCESS_LINK, '_blank');
  }

  onClickLearnMore() {
    window.open(AppSettings.CONTRIBUTORS_LEARN_MORE, '_blank');
  }

  onClickVersion(version) {
    this.selectedVersion = (environment.environment === 'prod') ? '' : version;
  }

  onClickVersionProceed() {
    if (this.selectedVersion === '') {
      this.error = 'Please select a EasyCLA Version.'
    } else {
      this.redirectAsPerTypeAndVersion(this.consoleType, this.selectedVersion);
    }
  }

  redirectAsPerTypeAndVersion(type: string, version: string) {
    let projectConsoleUrl = EnvConfig.default[AppSettings.PROJECT_CONSOLE_LINK] + '#/login';
    let corporateConsoleUrl = EnvConfig.default[AppSettings.CORPORATE_CONSOLE_LINK] + '#/login'

    if (version === '2') {
      projectConsoleUrl = EnvConfig.default[AppSettings.PROJECT_CONSOLE_LINK_V2];
      corporateConsoleUrl = EnvConfig.default[AppSettings.CORPORATE_CONSOLE_LINK_V2];
    }

    if (type === 'Projects') {
      window.open(projectConsoleUrl, '_self');
    }

    if (type === 'Organizations') {
      window.open(corporateConsoleUrl, '_self');
    }
  }

  onClickClose() {
    this.modalService.dismissAll();
  }
}
