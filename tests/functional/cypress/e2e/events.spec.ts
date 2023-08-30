import {validateApiResponse,validate_200_Status,getTokenKey} from '../support/commands'
describe("To Validate events are properly capture via API call", function () {
    //Reference api doc: https://api-gw.dev.platform.linuxfoundation.org/cla-service/v4/api-docs#tag/events
      const claEndpoint = `${Cypress.env("APP_URL")}cla-service/v4/events`;
      let claEndpointForNextKey="";
    let NextKey: string="";
    const foundationSFID='a09P000000DsNGsIAN'; //project name: easyAutom foundation
    const projectSfid='a09P000000DsNH2IAN'; //project name: easyAutom-child2  
    const companyID="f7c7ac9c-4dbf-4104-ab3f-6b38a26d82dc";
    const compProjectSFID="a092h000004x5tVAAQ";

    let bearerToken: string = null;
    before(() => { 
        if(bearerToken==null){
        getTokenKey(bearerToken);
        cy.window().then((win) => {
            bearerToken = win.localStorage.getItem('bearerToken');
          });
        }
    });

it("Get recent events of company and project - Record should Returns 200 Response", function () {
  claEndpointForNextKey=`${Cypress.env("APP_URL")}cla-service/v4/company/${companyID}/project/${compProjectSFID}/events`
    cy.request({
      method: 'GET',
      url: `${claEndpointForNextKey}`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
      
        // Validate specific data in the response
        let list=response.body;
        NextKey=list.NextKey;
        expect(list).to.have.property('NextKey'); 
        expect(list).to.have.property('ResultCount'); 
        expect(list).to.have.property('Events');           
        let Events = list.Events;
         // Assert that the response contains an array
        expect(Events).to.be.an('array');
          // Assert that the array has at least one item
        expect(Events.length).to.be.greaterThan(0);
            validateApiResponse("events/getCompanyProjectEvents.json",response);
        fetchNextRecords(claEndpointForNextKey,NextKey);  
        });
  });

it("Get events of foundation project - Record should Returns 200 Response", function () {
  claEndpointForNextKey=`${claEndpoint}/foundation/${foundationSFID}`
    cy.request({
      method: 'GET',
      url: `${claEndpointForNextKey}`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
      
        // Validate specific data in the response
        let list=response.body;
        NextKey=list.NextKey;
        expect(list).to.have.property('NextKey'); 
        expect(list).to.have.property('ResultCount'); 
        expect(list).to.have.property('Events');           
        let Events = list.Events;
         // Assert that the response contains an array
        expect(Events).to.be.an('array');
          // Assert that the array has at least one item
        expect(Events.length).to.be.greaterThan(0);
        // validateApiResponse("events/getFoundationEvents.json",list);
        fetchNextRecords(claEndpointForNextKey,NextKey);  
        });
  });

  it("Get events of child project - Record should Returns 200 Response", function () {
    claEndpointForNextKey=`${claEndpoint}/project/${projectSfid}`;
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/project/${projectSfid}`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
       
      let list=response.body;
        // Validate specific data in the response
        expect(list).to.have.property('NextKey'); 
        expect(list).to.have.property('ResultCount'); 
        expect(list).to.have.property('Events');           
        let Events = response.body.Events;
         // Assert that the response contains an array
        expect(Events).to.be.an('array');
          // Assert that the array has at least one item
        expect(Events.length).to.be.greaterThan(0);
          //To validate schema of response
       validateApiResponse("events/getProjectEvents",list)
       fetchNextRecords(claEndpointForNextKey,NextKey);  
        });
  });

  it.skip("Get List of recent events - requires Admin-level access - Record should Returns 200 Response", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/recent?pageSize=2`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
      
        // Validate specific data in the response
        let list=response.body;
        expect(list).to.have.property('NextKey'); 
        expect(list).to.have.property('ResultCount'); 
        expect(list).to.have.property('Events');           
        let Events = list.Events;
         // Assert that the response contains an array
        expect(Events).to.be.an('array');
          // Assert that the array has at least one item
        expect(Events.length).to.be.greaterThan(0);
          //To validate schema of response
             validateApiResponse("events/getProjectEvents.json",list)
        });
  });

  it("Download all the events for the foundation as a CSV document - Record should Returns 200 Response", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/foundation/${foundationSFID}/csv`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
        });
  });

  it("Download all the events for the project as a CSV document - Record should Returns 200 Response", function () {
    cy.request({
      method: 'GET',
      url: `${claEndpoint}/project/${projectSfid}/csv`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
        });
  });

  function fetchNextRecords(URL,NextKey){
    if(NextKey!==undefined){
    cy.request({
      method: 'GET',
      url: `${URL}?nextKey=${NextKey}&pageSize=50`,
      auth: {
        'bearer': bearerToken,
      }
    }).then((response) => {
      validate_200_Status(response);
      
        // Validate specific data in the response
        let updatedNextKey=response.body.NextKey;
       if(updatedNextKey!==undefined){
        fetchNextRecords(URL,updatedNextKey);
       }
      });
    }
  }
})