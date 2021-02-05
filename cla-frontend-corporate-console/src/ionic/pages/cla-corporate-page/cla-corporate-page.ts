// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

import { Component } from '@angular/core';
import { IonicPage, NavController, NavParams } from 'ionic-angular';
import { ClaService } from '../../services/cla.service';
import { generalConstants } from '../../constant/general';

@IonicPage({
  segment: 'project/:projectId',
})
@Component({
  selector: 'cla-corporate-page',
  templateUrl: 'cla-corporate-page.html',
})
export class ClaCorporatePage {
  projectId: string;
  loading: any;
  company: any;
  project: any;
  signatureIntent: any;
  activeSignatures: boolean = true; // we assume true until otherwise
  signature: any;
  error: any = false;
  expanded: boolean = true;

  constructor(
    public navCtrl: NavController,
    public navParams: NavParams,
    private claService: ClaService,
  ) {
    this.getDefaults();
    this.projectId = navParams.get('projectId');
    this.company = navParams.get('company');
  }

  getDefaults() {
    this.project = {
      project_name: '',
    };
    this.signature = {
      sign_url: '',
    };
  }

  ngOnInit() {
    this.loading = true;
    this.getProject(this.projectId);
    this.postSignatureRequest();
  }

  getProject(projectId) {
    this.claService.getProject(projectId).subscribe((response) => {
      this.project = response;
    });
  }

  getReturnUrl() {
    return window.location.protocol + '//' + window.location.host + '/#/company/' + this.company.company_id;
  }

  postSignatureRequest() {
    let signatureRequest = {
      project_id: this.projectId,
      company_id: this.company.company_id,
      send_as_email: false,
      authority_name: localStorage.getItem('user_name'),
      authority_email: localStorage.getItem('user_email'),
      return_url: this.getReturnUrl(),
    };

    this.claService.postCorporateSignatureRequest(signatureRequest).subscribe((response) => {
      this.loading = false;
      if (response.errors) {
        this.error = response;
      }
      this.signature = response;
    });
  }

  openClaAgreement() {
    if (!this.signature.sign_url) {
      return;
    }
    window.open(this.signature.sign_url, '_self');
  }

  createTicket() {
    window.open(generalConstants.createTicketURL, '_blank');
  }

  onClickToggle(hasExpanded) {
    this.expanded = hasExpanded;
  }
}
